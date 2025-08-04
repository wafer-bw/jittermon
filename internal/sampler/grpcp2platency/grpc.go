package grpcp2platency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/grpcp2platency/internal/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Recorder interface {
	recorder.Recorder
}

type ClientPoller interface {
	pollpb.PollServiceClient
}

const (
	ClientName string = "grpc_p2platency_client"
	ServerName string = "grpc_p2platency_server"

	defaultInterval          time.Duration = 1 * time.Second
	defaultTimeout           time.Duration = defaultInterval * time.Duration(2)
	defaultProto             string        = "tcp"
	defaultReflectionEnabled bool          = true

	maxConnectionIdle time.Duration = 5 * time.Minute
)

var (
	defaultLog           = slog.New(slog.DiscardHandler)
	defaultServerOptions = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle})}
	defaultDialOptions   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
)

type ClientOption func(*Client) error

func WithClientID(id string) ClientOption {
	return func(c *Client) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		c.id = id
		return nil
	}
}

func WithClientInterval(interval time.Duration) ClientOption {
	return func(c *Client) error {
		if interval <= 0 {
			return nil
		}
		c.interval = interval
		return nil
	}
}

func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) error {
		if timeout <= 0 {
			return nil
		}
		c.timeout = timeout
		return nil
	}
}

func WithClientLog(log *slog.Logger) ClientOption {
	return func(c *Client) error {
		if log == nil {
			return nil
		}
		c.log = log
		return nil
	}
}

func WithClientDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *Client) error {
		c.dialOptions = opts
		return nil
	}
}

type Client struct {
	id          string
	address     string
	interval    time.Duration
	timeout     time.Duration
	recorder    Recorder
	dialOptions []grpc.DialOption
	log         *slog.Logger
	conn        ClientPoller
}

func NewClient(address string, recorder Recorder, options ...ClientOption) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	} else if recorder == nil {
		return nil, fmt.Errorf("recorder cannot be nil")
	}

	c := &Client{
		address:     address,
		recorder:    recorder,
		id:          littleid.New(),
		interval:    defaultInterval,
		timeout:     defaultTimeout,
		dialOptions: defaultDialOptions,
		log:         defaultLog,
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	c.log = c.log.With("name", ClientName, "id", c.id, "address", c.address)

	return c, nil
}

func (c Client) Poll(ctx context.Context) error {
	labels := recorder.Labels{{K: "src", V: c.id}, {K: "dst", V: c.address}}

	start := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(c.id)
	req.SetTimestamp(timestamppb.New(start))
	pCtx, cancel := context.WithTimeout(ctx, c.timeout) // TODO: determine what to set this to, too early and we report non-lost packets.
	defer cancel()
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	rsp, err := c.conn.Poll(pCtx, req)
	if err != nil {
		c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		return err
	}
	end := time.Now()

	rtt := end.Sub(start)
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeRTT, Val: rtt, Labels: labels})

	dstID := rsp.GetId()
	if dstID == "" {
		return fmt.Errorf("no id in response")
	}
	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeUpstreamJitter, Val: jit, Labels: labels})

	return nil
}

func (c *Client) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.InfoContext(ctx, "starting", "interval", c.interval)

	if c.conn == nil { // can exist already in tests.
		conn, err := grpc.NewClient(c.address, c.dialOptions...)
		if err != nil {
			return err
		}
		defer conn.Close()
		c.conn = pollpb.NewPollServiceClient(conn)
	}

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				c.log.WarnContext(ctx, "poll failed", "err", err)
				continue
			}
		case <-ctx.Done():
			c.log.WarnContext(ctx, "context done, stopping", "err", ctx.Err())
			return ctx.Err()
		}
	}
}

type ServerOption func(*Server) error

func WithServerID(id string) ServerOption {
	return func(s *Server) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		s.id = id
		return nil
	}
}

func WithServerProto(proto string) ServerOption {
	return func(s *Server) error {
		if proto == "" {
			return nil
		}
		s.proto = proto
		return nil
	}
}

func WithServerOptions(opts ...grpc.ServerOption) ServerOption {
	return func(s *Server) error {
		s.serverOptions = opts
		return nil
	}
}

func WithServerReflection(enabled bool) ServerOption {
	return func(s *Server) error {
		s.reflectionEnabled = enabled
		return nil
	}
}

func WithServerLog(log *slog.Logger) ServerOption {
	return func(s *Server) error {
		if log == nil {
			return nil
		}
		s.log = log
		return nil
	}
}

type Server struct {
	id                string
	address           string
	proto             string
	serverOptions     []grpc.ServerOption
	reflectionEnabled bool
	recorder          Recorder
	jitter            *jitter.Buffer
	log               *slog.Logger
	server            *grpc.Server

	pollpb.UnimplementedPollServiceServer
}

func NewServer(address string, recorder Recorder, options ...ServerOption) (*Server, error) {
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	} else if recorder == nil {
		return nil, fmt.Errorf("recorder cannot be nil")
	}

	s := &Server{
		address:           address,
		recorder:          recorder,
		id:                littleid.New(),
		jitter:            &jitter.Buffer{},
		proto:             defaultProto,
		serverOptions:     defaultServerOptions,
		reflectionEnabled: defaultReflectionEnabled,
		log:               defaultLog,
	}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	s.log = s.log.With("name", ServerName, "id", s.id, "address", s.address)

	return s, nil
}

func (s Server) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()
	resp := &pollpb.PollResponse{}
	resp.SetId(s.id)

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	jitter, ok := s.jitter.Interarrival(srcID, sentAt, now)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := recorder.Labels{{K: "src", V: srcID}, {K: "dst", V: s.id}}
	s.recorder.Record(ctx, recorder.Sample{Time: now, Type: recorder.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (s *Server) Run(ctx context.Context) error {
	s.log.InfoContext(ctx, "starting")

	s.server = grpc.NewServer(s.serverOptions...)
	pollpb.RegisterPollServiceServer(s.server, s)
	if s.reflectionEnabled {
		reflection.Register(s.server)
	}
	defer s.server.Stop()

	listener, err := net.Listen(s.proto, s.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	errCh := make(chan error)
	go func() {
		if err := s.server.Serve(listener); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.log.WarnContext(ctx, "context done, stopping", "err", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		s.log.ErrorContext(ctx, "server failed", "err", err)
		return err
	}
}
