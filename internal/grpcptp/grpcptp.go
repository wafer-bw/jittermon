package grpcptp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	pollpb "github.com/wafer-bw/jittermon/internal/gen/go/poll/v1"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClientPoller interface {
	pollpb.PollServiceClient
}

const (
	name                     string        = "grpc ptp client"
	defaultInterval          time.Duration = 1 * time.Second
	defaultTimeout           time.Duration = defaultInterval * time.Duration(2)
	defaultProto             string        = "tcp"
	defaultReflectionEnabled bool          = true
	defaultPollGrace         int           = 1
	maxConnectionIdle        time.Duration = 5 * time.Minute
)

var (
	defaultLog           = slog.New(slog.DiscardHandler)
	defaultServerOptions = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle})}
	defaultDialOptions   = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
)

type ClientOption func(*Client) error

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
	dialOptions []grpc.DialOption
	log         *slog.Logger
	attributes  []attribute.KeyValue
	conn        ClientPoller
	pollGrace   int
}

func NewClient(id string, address string, options ...ClientOption) (*Client, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	} else if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	c := &Client{
		id:          id,
		address:     address,
		interval:    defaultInterval,
		timeout:     defaultTimeout,
		dialOptions: defaultDialOptions,
		log:         defaultLog,
		pollGrace:   defaultPollGrace,
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	c.attributes = []attribute.KeyValue{}

	return c, nil
}

func (c Client) Poll(ctx context.Context) error {
	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, c.id),
		attribute.String(otel.DestinationLabelName, c.address),
	)

	start := time.Now()

	req := pollpb.PollRequest_builder{
		Id:        &c.id,
		Timestamp: timestamppb.New(start),
	}.Build()

	pCtx, cancel := context.WithTimeout(ctx, c.timeout) // TODO: determine what to set this to, too early and we report non-lost packets.
	defer cancel()

	otel.SentPacketsCounter.Add(ctx, 1, attributes)
	c.log.DebugContext(ctx, otel.SentPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)
	rsp, err := c.conn.Poll(pCtx, req)
	if err != nil {
		otel.LostPacketsCounter.Add(ctx, 1, attributes)
		c.log.DebugContext(ctx, otel.LostPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)
		return err
	}

	end := time.Now()
	rtt := end.Sub(start)
	otel.PingHistogram.Record(ctx, rtt.Seconds(), attributes)
	c.log.DebugContext(ctx, otel.PingMetricName, "value", strconv.FormatFloat(rtt.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)

	dstID := rsp.GetId()
	if dstID == "" {
		return fmt.Errorf("no id in response")
	}
	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()
	otel.UpstreamJitterHistogram.Record(ctx, jit.Seconds(), attributes)
	c.log.DebugContext(ctx, otel.UpstreamJitterMetricName, "value", strconv.FormatFloat(jit.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.InfoContext(ctx, "starting", "name", name, "address", c.address, "interval", c.interval)

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
				if c.pollGrace > 0 {
					c.pollGrace--
				} else {
					c.log.ErrorContext(ctx, "poll failed", "name", name, "err", err)
				}
				continue
			}
		case <-ctx.Done():
			c.log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
			return ctx.Err()
		}
	}
}

// TODO: implement.
func (c *Client) Stop(ctx context.Context) error {
	return nil
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
	jitter            *jitter.Buffer
	log               *slog.Logger
	server            *grpc.Server

	pollpb.UnimplementedPollServiceServer
}

func NewServer(id string, address string, options ...ServerOption) (*Server, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	} else if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	s := &Server{
		id:                id,
		address:           address,
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

	return s, nil
}

func (s Server) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	start := time.Now()

	resp := pollpb.PollResponse_builder{Id: &s.id}.Build()

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, srcID),
		attribute.String(otel.DestinationLabelName, s.id),
	)

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	jitter, ok := s.jitter.Interarrival(srcID, sentAt, start)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	otel.DownstreamJitterHistogram.Record(ctx, jitter.Seconds(), attributes)
	s.log.DebugContext(ctx, otel.DownstreamJitterMetricName, "value", strconv.FormatFloat(jitter.Seconds(), 'f', 6, 64), otel.SourceLabelName, srcID, otel.DestinationLabelName, s.id)

	return resp, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.log.InfoContext(ctx, "starting", "name", name, "address", s.address)

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
		s.log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		s.log.ErrorContext(ctx, "server failed", "name", name, "err", err)
		return err
	}
}

// TODO: implement.
func (c *Server) Stop(ctx context.Context) error {
	return nil
}
