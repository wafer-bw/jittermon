// TODO: docstring.
package p2platency

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency/internal/grpcpeer"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency/internal/grpcpeer/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var _ graceful.Runner = (*Peer)(nil)

const (
	SamplerName     string        = "p2platency"
	DefaultInterval time.Duration = 1 * time.Second
)

const (
	defaultMode       mode          = modeGRPC
	defaultProto      string        = "tcp"
	maxConnectionIdle time.Duration = 5 * time.Minute
)

var (
	defaulGRPCClientOpts  []grpc.DialOption   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	defaultGRPCServerOpts []grpc.ServerOption = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle})}
	defaultGRPCReflection bool                = true
	defaultLogger         *slog.Logger        = slog.New(slog.DiscardHandler)
)

type Recorder interface {
	recorder.Recorder
}

type mode uint8

const (
	modeGRPC mode = iota // only supported mode for now
)

type Peer struct {
	id            string
	sendAddresses []string
	listenAddress string
	interval      time.Duration
	recorder      Recorder
	mode          mode
	grpc          grpcConfig
	log           *slog.Logger
	group         graceful.Group
	stoppedCh     chan struct{}
	doneCh        chan struct{}

	pollpb.UnimplementedPollServiceServer
}

type grpcConfig struct {
	Proto         string
	ClientOptions []grpc.DialOption
	ServerOptions []grpc.ServerOption
	Reflection    bool
}

type Option func(*Peer) error

func WithID(id string) Option {
	return func(p *Peer) error {
		if id == "" {
			return nil
		}
		p.id = strings.TrimSpace(id)
		return nil
	}
}

func WithInterval(interval time.Duration) Option {
	return func(p *Peer) error {
		if interval <= 0 {
			return nil
		}
		p.interval = interval
		return nil
	}
}

func WithRecorder(recorder rec.Recorder) Option {
	return func(p *Peer) error {
		p.recorder = recorder
		return nil
	}
}

func WithLog(log *slog.Logger) Option {
	return func(p *Peer) error {
		p.log = log
		return nil
	}
}

func WithSendAddresses(addrs ...string) Option {
	return func(p *Peer) error {
		p.sendAddresses = addrs
		return nil
	}
}

func WithListenAddress(addr string) Option {
	return func(p *Peer) error {
		p.listenAddress = addr
		return nil
	}
}

func WithGRPC(proto string, clientOpts []grpc.DialOption, serverOpts []grpc.ServerOption, reflection bool) Option {
	return func(p *Peer) error {
		p.mode = modeGRPC
		p.grpc.Proto = proto
		p.grpc.ServerOptions = serverOpts
		p.grpc.ClientOptions = clientOpts
		p.grpc.Reflection = reflection
		return nil
	}
}

// TODO: WithEnv option.

func NewPeer(options ...Option) (*Peer, error) {
	p := &Peer{
		id:       littleid.New(),
		interval: DefaultInterval,
		recorder: rec.NoOp,
		mode:     defaultMode,
		grpc: grpcConfig{
			Proto:         defaultProto,
			ServerOptions: defaultGRPCServerOpts,
			ClientOptions: defaulGRPCClientOpts,
			Reflection:    defaultGRPCReflection,
		},
		log:       defaultLogger,
		stoppedCh: make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	p.group = graceful.Group{}
	for _, addr := range p.sendAddresses {
		client := &grpcpeer.Client{
			ID:            p.id,
			Address:       addr,
			Interval:      p.interval,
			Recorder:      p.recorder,
			ClientOptions: p.grpc.ClientOptions,
			Log:           p.log,
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
		}
		p.group = append(p.group, client)
	}

	if p.listenAddress != "" {
		server := &grpcpeer.Server{
			ID:                      p.id,
			Address:                 p.listenAddress,
			Proto:                   p.grpc.Proto,
			ServerOptions:           p.grpc.ServerOptions,
			ServerReflectionEnabled: p.grpc.Reflection,
			Recorder:                p.recorder,
			RequestBuffers:          &jitter.Buffer{},
			Log:                     p.log,
			StartedCh:               make(chan struct{}),
			StoppedCh:               make(chan struct{}),
		}
		p.group = append(p.group, server)
	}

	return p, nil
}

func (p *Peer) Start(ctx context.Context) error {
	defer close(p.stoppedCh)
	return p.group.Start(ctx)
}

func (p *Peer) Stop(ctx context.Context) error {
	select {
	case <-p.stoppedCh:
	case <-ctx.Done():
	}

	// TODO: this shouldn't be used, it should just be based on the ctx passed
	// to stop.
	var timeout = 60 * time.Second
	return p.group.Stop(ctx, timeout)
}
