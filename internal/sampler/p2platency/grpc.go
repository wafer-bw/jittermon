package p2platency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency/internal/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: audit naming and name values.
// TODO: consider how to do protocol split.

var _ graceful.Runner = &Peer{}

const (
	SamplerName             string        = "p2platency"
	DefaultInterval         time.Duration = 1 * time.Second
	DefaultProto            string        = "tcp"
	DefaultServerReflection bool          = true

	clientName        string        = "grpc_p2platency_client"
	serverName        string        = "grpc_p2platency_server"
	maxConnectionIdle time.Duration = 5 * time.Minute
)

var (
	DefaulDialOpts    []grpc.DialOption   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	DefaultServerOpts []grpc.ServerOption = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle})}
	DefaultLogger     *slog.Logger        = slog.New(slog.DiscardHandler)
)

type Recorder interface {
	rec.Recorder
}

type GRPCClientPoller interface {
	pollpb.PollServiceClient
}

type Peer struct {
	id            string
	sendAddresses []string
	listenAddress string
	interval      time.Duration
	recorder      Recorder
	proto         string
	dialOptions   []grpc.DialOption
	serverOptions []grpc.ServerOption
	reflection    bool
	log           *slog.Logger
	group         graceful.Group
	stoppedCh     chan struct{}
	doneCh        chan struct{}

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

func WithProto(proto string) Option {
	return func(p *Peer) error {
		if proto == "" {
			return nil
		}
		p.proto = proto
		return nil
	}
}

func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(p *Peer) error {
		p.dialOptions = opts
		return nil
	}
}

func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(p *Peer) error {
		p.serverOptions = opts
		return nil
	}
}

func WithServerReflection(enabled bool) Option {
	return func(p *Peer) error {
		p.reflection = enabled
		return nil
	}
}

func NewPeer(options ...Option) (*Peer, error) {
	p := &Peer{
		id:            littleid.New(),
		interval:      DefaultInterval,
		recorder:      rec.NoOp,
		proto:         DefaultProto,
		serverOptions: DefaultServerOpts,
		dialOptions:   DefaulDialOpts,
		reflection:    DefaultServerReflection,
		log:           DefaultLogger,
		stoppedCh:     make(chan struct{}),
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

	p.group = graceful.Group{}
	for _, addr := range p.sendAddresses {
		client := &Client{
			ID:          p.id,
			Address:     addr,
			Interval:    p.interval,
			Recorder:    p.recorder,
			DialOptions: p.dialOptions,
			Log:         p.log,
			StopCh:      make(chan struct{}),
			StoppedCh:   make(chan struct{}),
		}
		p.group = append(p.group, client)
	}

	if p.listenAddress != "" {
		server := &Server{
			ID:                      p.id,
			Address:                 p.listenAddress,
			Proto:                   p.proto,
			ServerOptions:           p.serverOptions,
			ServerReflectionEnabled: p.reflection,
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

	return p.group.Stop(ctx)
}

type Client struct {
	ID          string
	Address     string
	Interval    time.Duration
	Recorder    Recorder
	DialOptions []grpc.DialOption
	Log         *slog.Logger
	StopCh      chan struct{}
	StoppedCh   chan struct{}
	Client      GRPCClientPoller
}

func (c Client) Poll(ctx context.Context) error {
	start := time.Now()
	labels := rec.Labels{{K: "src", V: c.ID}, {K: "dst", V: c.Address}}

	req := &pollpb.PollRequest{}
	req.SetId(c.ID)
	req.SetTimestamp(timestamppb.New(start))

	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	rsp, err := c.Client.Poll(ctx, req)
	if err != nil {
		c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		c.Log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(start)

	dstID := rsp.GetId()
	if dstID == "" {
		c.Log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		c.Log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()

	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeUpstreamJitter, Val: jit, Labels: labels})
	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: labels})

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	c.Log = c.Log.With("id", c.ID, "name", clientName, "address", c.Address)
	c.Log.Info("starting")

	defer close(c.StoppedCh)

	if c.Client == nil {
		conn, err := grpc.NewClient(c.Address, c.DialOptions...)
		if err != nil {
			return err
		}
		defer conn.Close()
		c.Client = pollpb.NewPollServiceClient(conn)
	}

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				c.Log.Warn("do poll failed", "err", err)
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-c.StopCh:
			return nil
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.Log.Debug("stopping")

	close(c.StopCh)

	select {
	case <-c.StoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", clientName, c.ID, ctx.Err())
	}
}

type Server struct {
	ID                      string
	Address                 string
	Proto                   string
	ServerOptions           []grpc.ServerOption
	ServerReflectionEnabled bool
	Recorder                Recorder
	RequestBuffers          *jitter.Buffer
	Log                     *slog.Logger
	StartedCh               chan struct{}
	StoppedCh               chan struct{}

	Server *grpc.Server

	pollpb.UnimplementedPollServiceServer
}

func (s Server) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()
	resp := &pollpb.PollResponse{}
	resp.SetId(s.ID)

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	jitter, ok := s.RequestBuffers.Interarrival(srcID, sentAt, now)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := rec.Labels{{K: "src", V: srcID}, {K: "dst", V: s.ID}}
	s.Recorder.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.Log = s.Log.With("id", s.ID, "name", serverName, "address", s.Address)
	s.Log.Info("starting")

	defer close(s.StoppedCh)

	s.Server = grpc.NewServer(s.ServerOptions...)
	pollpb.RegisterPollServiceServer(s.Server, s)
	if s.ServerReflectionEnabled {
		reflection.Register(s.Server)
	}

	var err error
	listener, err := net.Listen(s.Proto, s.Address)
	if err != nil {
		return err
	}
	defer listener.Close()

	close(s.StartedCh)

	return s.Server.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.Log.Debug("stopping")

	gracefullyStoppedCh := make(chan struct{})
	go func() {
		select {
		case <-s.StartedCh:
		case <-s.StoppedCh:
		case <-ctx.Done():
			return
		}

		s.Server.GracefulStop()
		close(gracefullyStoppedCh)
	}()

	select {
	case <-gracefullyStoppedCh:
		return nil
	case <-ctx.Done():
		s.Server.Stop()
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", serverName, s.ID, ctx.Err())
	}
}
