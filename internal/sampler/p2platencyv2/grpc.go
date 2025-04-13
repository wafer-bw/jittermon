// TODO: docstring
package p2platencyv2

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platencyv2/internal/grpcpeer"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platencyv2/internal/grpcpeer/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var _ graceful.Runner = (*GRPC)(nil)

const (
	SamplerName     string        = "p2platency"
	DefaultInterval time.Duration = 1 * time.Second
)

const (
	defaultProto       string        = "tcp"
	defaultMaxConnIdle time.Duration = 5 * time.Minute
)

var (
	defaulGRPCClientOpts  []grpc.DialOption   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	defaultGRPCServerOpts []grpc.ServerOption = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: defaultMaxConnIdle})}
	defaultGRPCReflection bool                = true
	defaultLogger         *slog.Logger        = slog.New(slog.DiscardHandler)
)

type GRPC struct {
	id            string
	listenAddress string
	sendAddresses []string
	interval      time.Duration
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

type Option func(*GRPC) error

func WithID(id string) Option {
	return func(p *GRPC) error {
		if id == "" {
			return nil
		}
		p.id = strings.TrimSpace(id)
		return nil
	}
}

func WithInterval(interval time.Duration) Option {
	return func(p *GRPC) error {
		if interval <= 0 {
			return nil
		}
		p.interval = interval
		return nil
	}
}

func WithLog(log *slog.Logger) Option {
	return func(p *GRPC) error {
		if log == nil {
			return nil
		}
		p.log = log
		return nil
	}
}

func WithListenAddress(addr string) Option {
	return func(p *GRPC) error {
		p.listenAddress = addr
		return nil
	}
}

func WithSendAddresses(addrs ...string) Option {
	return func(p *GRPC) error {
		p.sendAddresses = addrs
		return nil
	}
}

func WithProto(proto string) Option {
	return func(p *GRPC) error {
		if proto == "" {
			return nil
		}
		p.grpc.Proto = strings.TrimSpace(proto)
		return nil
	}
}

func WithClientOptions(opts ...grpc.DialOption) Option {
	return func(p *GRPC) error {
		p.grpc.ClientOptions = opts
		return nil
	}
}

func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(p *GRPC) error {
		p.grpc.ServerOptions = opts
		return nil
	}
}

func WithServerReflection(enabled bool) Option {
	return func(p *GRPC) error {
		p.grpc.Reflection = enabled
		return nil
	}
}

// TODO: WithEnv option.

func NewPeer(options ...Option) (*GRPC, error) {
	p := &GRPC{
		id:       littleid.New(),
		interval: DefaultInterval,
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
			ClientOptions: p.grpc.ClientOptions,
			Log:           p.log,
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
			SampleCh:      make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
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
			RequestBuffers:          &jitter.Buffer{},
			Log:                     p.log,
			StartedCh:               make(chan struct{}),
			StoppedCh:               make(chan struct{}),
			SampleCh:                make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}
		p.group = append(p.group, server)
	}

	return p, nil
}

func (p *GRPC) Start(ctx context.Context) error {
	defer close(p.stoppedCh)
	return p.group.Start(ctx)
}

func (p *GRPC) Stop(ctx context.Context) error {
	select {
	case <-p.stoppedCh:
	case <-ctx.Done():
	}

	// TODO: this shouldn't be used, it should just be based on the ctx passed
	// to stop.
	var timeout = 60 * time.Second
	return p.group.Stop(ctx, timeout)
}
