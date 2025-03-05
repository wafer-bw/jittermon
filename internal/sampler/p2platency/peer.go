package p2platency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	sendAddr                string
	listenAddr              string
	interval                time.Duration
	recorder                Recorder
	requestBuffers          jitter.HostPacketBuffers // TODO: accept interface.
	proto                   string
	server                  *grpc.Server
	serverOptions           []grpc.ServerOption
	serverReflectionEnabled bool
	client                  pollpb.PollServiceClient
	clientOptions           []grpc.DialOption
	clientConn              *grpc.ClientConn
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

func WithSendAddress(addr string) Option {
	return func(p *Peer) error {
		p.sendAddr = addr
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

	p.log = p.log.With(
		"id", p.id,
		"name", name,
	)

	return p, nil
}

func (p Peer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()
	resp := &pollpb.PollResponse{}
	resp.SetId(p.id)

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	p.requestBuffers.Sample(srcID, jitter.Packet{S: sentAt, R: now})
	jitter, ok := p.requestBuffers.Jitter(srcID)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := rec.Labels{{K: "src", V: srcID}, {K: "dst", V: p.id}}
	p.recorder.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (p Peer) DoPoll(ctx context.Context) error {
	start := time.Now()
	labels := rec.Labels{{K: "src", V: p.id}, {K: "dst", V: p.sendAddr}}

	req := &pollpb.PollRequest{}
	req.SetId(p.id)
	req.SetTimestamp(timestamppb.New(start))

	p.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	rsp, err := p.client.Poll(ctx, req)
	if err != nil {
		p.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		p.log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(start)

	dstID := rsp.GetId()
	if dstID == "" {
		p.log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		p.log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()

	p.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeUpstreamJitter, Val: jit, Labels: labels})
	p.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: labels})

	return nil
}

func (p Peer) Start(ctx context.Context) error {
	p.log.Info("starting")
	defer close(p.doneCh)

	var err error
	p.clientConn, err = grpc.NewClient(p.sendAddr, p.clientOptions...)
	if err != nil {
		return err
	}
	p.client = pollpb.NewPollServiceClient(p.clientConn)
	defer p.clientConn.Close() // TODO: close here or in stop?

	p.server = grpc.NewServer(p.serverOptions...)
	pollpb.RegisterPollServiceServer(p.server, p)
	if p.serverReflectionEnabled {
		reflection.Register(p.server)
	}

	listener, err := net.Listen(p.proto, p.listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close() // TODO: close here or in stop?

	p.ticker = time.NewTicker(p.interval)
	defer p.ticker.Stop()

	close(p.startedCh)

	// TODO: wrap following blocks in a graceful.Group?

	go func() { _ = p.server.Serve(listener) }()

	for {
		select {
		case <-p.ticker.C:
			if err := p.DoPoll(ctx); err != nil {
				p.log.Warn("do poll failed", "err", err)
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		}
	}
}

func (p Peer) Stop(ctx context.Context) error {
	<-p.startedCh // wait for [Client.Start] to finish
	_ = p.clientConn.Close()

	close(p.stopCh)

	select {
	case <-p.doneCh:
		// fallthrough
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", name, p.id, ctx.Err())
	}

	return nil
}
