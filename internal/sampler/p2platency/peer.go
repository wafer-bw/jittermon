package p2platency

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency/internal/grpcpeer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	name              string        = "grpc_p2p_latency"
	defaultProto      string        = "tcp"
	maxConnectionIdle time.Duration = 5 * time.Minute

	DefaultInterval time.Duration = 1 * time.Second
)

var (
	defaultGRPCServerOpts []grpc.ServerOption = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle})}
	defaulGRPCClientOpts  []grpc.DialOption   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	defaultLogger         *slog.Logger        = slog.New(slog.DiscardHandler)
)

type Recorder interface {
	recorder.Recorder
}

type Peer struct {
	id                      string
	sendAddrs               []string
	listenAddr              string
	interval                time.Duration
	recorder                Recorder
	requestBuffers          jitter.HostPacketBuffers // TODO: accept interface.
	proto                   string
	server                  *grpc.Server
	serverOptions           []grpc.ServerOption
	serverReflectionEnabled bool
	clients                 map[string]pollpb.PollServiceClient
	clientConns             map[string]*grpc.ClientConn
	clientOptions           []grpc.DialOption
	log                     *slog.Logger
	ticker                  *time.Ticker
	startedCh               chan struct{} // TODO: is this needed?
	stopCh                  chan struct{} // TODO: is this needed?
	doneCh                  chan struct{} // TODO: is this needed?

	pollpb.UnimplementedPollServiceServer
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
		p.sendAddrs = addrs
		return nil
	}
}

func WithListenAddress(addr string) Option {
	return func(p *Peer) error {
		p.listenAddr = addr
		return nil
	}
}

func WithServerReflectionEnabled(b bool) Option {
	return func(p *Peer) error {
		p.serverReflectionEnabled = b
		return nil
	}
}

func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(p *Peer) error {
		p.serverOptions = opts
		return nil
	}
}

func WithGRPCClientOptions(opts ...grpc.DialOption) Option {
	return func(p *Peer) error {
		p.clientOptions = opts
		return nil
	}
}

// TODO: WithEnv option.

func NewPeer(options ...Option) (*Peer, error) {
	p := &Peer{
		id:            littleid.New(),
		interval:      DefaultInterval,
		recorder:      rec.NoOp,
		proto:         defaultProto,
		serverOptions: defaultGRPCServerOpts,
		clientOptions: defaulGRPCClientOpts,
		log:           defaultLogger,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func (p *Peer) Start(ctx context.Context) error {
	group := graceful.Group{}
	for _, addr := range p.sendAddrs {
		client := &grpcpeer.Client{
			ID:            p.id,
			Address:       addr,
			Interval:      p.interval,
			Recorder:      p.recorder,
			ClientOptions: p.clientOptions,
			Log:           p.log,
		}
		group = append(group, client)
	}

	server := &grpcpeer.Server{
		ID:                      p.id,
		Address:                 p.listenAddr,
		Proto:                   p.proto,
		ServerOptions:           p.serverOptions,
		ServerReflectionEnabled: p.serverReflectionEnabled,
		Recorder:                p.recorder,
		Log:                     p.log,
	}
	group = append(group, server)

	return group.Start(ctx)
}

func (p *Peer) Stop(ctx context.Context) error {
	return nil
}
