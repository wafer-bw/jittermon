package grpcptp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
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

type Int64Counter interface {
	Add(ctx context.Context, incr int64, options ...metric.AddOption)
}

type Float64Histogram interface {
	Record(ctx context.Context, incr float64, options ...metric.RecordOption)
}

const (
	name                     string        = "grpc ptp client"
	defaultInterval          time.Duration = 1 * time.Second
	defaultMaxConnectionIdle time.Duration = 5 * time.Minute
	defaultProto             string        = "tcp"
	startingPollGrace        int           = 1
)

var defaultLog = slog.New(slog.DiscardHandler)

type Client struct {
	ID                      string
	Address                 string
	SentPacketsCounter      Int64Counter
	LostPacketsCounter      Int64Counter
	PingHistogram           Float64Histogram
	UpstreamJitterHistogram Float64Histogram

	Interval    time.Duration // uses [defaultInterval] if not set.
	Timeout     time.Duration // uses [Client.Interval] if not set.
	DialOptions []grpc.DialOption
	Log         *slog.Logger
	Conn        ClientPoller

	pollGrace int
}

func (c *Client) Poll(ctx context.Context) error {
	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, c.ID),
		attribute.String(otel.DestinationLabelName, c.Address),
	)

	start := time.Now()
	if c.Log == nil {
		c.Log = defaultLog
	}

	req := pollpb.PollRequest_builder{
		Id:        &c.ID,
		Timestamp: timestamppb.New(start),
	}.Build()

	pCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	c.SentPacketsCounter.Add(ctx, 1, attributes)
	c.Log.DebugContext(ctx, otel.SentPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)
	rsp, err := c.Conn.Poll(pCtx, req)
	if err != nil {
		c.LostPacketsCounter.Add(ctx, 1, attributes)
		c.Log.DebugContext(ctx, otel.LostPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)
		return err
	}

	end := time.Now()
	rtt := end.Sub(start)
	c.PingHistogram.Record(ctx, rtt.Seconds(), attributes)
	c.Log.DebugContext(ctx, otel.PingMetricName, "value", strconv.FormatFloat(rtt.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)

	dstID := rsp.GetId()
	if dstID == "" {
		return fmt.Errorf("no id in response")
	}
	jitterPb := rsp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()
	c.UpstreamJitterHistogram.Record(ctx, jit.Seconds(), attributes)
	c.Log.DebugContext(ctx, otel.UpstreamJitterMetricName, "value", strconv.FormatFloat(jit.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	if c.ID == "" {
		return fmt.Errorf("%s client start: id cannot be empty", name)
	} else if c.Address == "" {
		return fmt.Errorf("%s client start: address cannot be empty", name)
	} else if c.Interval <= 0 {
		c.Interval = defaultInterval
	}

	if c.Timeout <= 0 {
		c.Timeout = c.Interval
	}
	if c.Log == nil {
		c.Log = defaultLog
	}
	if len(c.DialOptions) == 0 {
		c.DialOptions = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	c.pollGrace = startingPollGrace

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	c.Log.InfoContext(ctx, "starting", "name", name, "address", c.Address, "interval", c.Interval)

	if c.Conn == nil { // tests provide conn, normally we create one.
		conn, err := grpc.NewClient(c.Address, c.DialOptions...)
		if err != nil {
			return err
		}
		defer conn.Close()
		c.Conn = pollpb.NewPollServiceClient(conn)
	}

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				if c.pollGrace > 0 {
					c.pollGrace--
				} else {
					c.Log.ErrorContext(ctx, "poll failed", "name", name, "err", err)
				}
				continue
			}
		case <-ctx.Done():
			c.Log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
			return ctx.Err()
		}
	}
}

type Server struct {
	ID                        string
	Address                   string
	DownstreamJitterHistogram Float64Histogram

	Proto         string
	ServerOptions []grpc.ServerOption
	Log           *slog.Logger
	Server        *grpc.Server

	jitter *jitter.Buffer

	pollpb.UnimplementedPollServiceServer
}

func (s *Server) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	start := time.Now()
	if s.jitter == nil {
		s.jitter = &jitter.Buffer{}
	}
	if s.Log == nil {
		s.Log = defaultLog
	}

	resp := pollpb.PollResponse_builder{Id: &s.ID}.Build()

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, srcID),
		attribute.String(otel.DestinationLabelName, s.ID),
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

	s.DownstreamJitterHistogram.Record(ctx, jitter.Seconds(), attributes)
	s.Log.DebugContext(ctx, otel.DownstreamJitterMetricName, "value", strconv.FormatFloat(jitter.Seconds(), 'f', 6, 64), otel.SourceLabelName, srcID, otel.DestinationLabelName, s.ID)

	return resp, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.ID == "" {
		return fmt.Errorf("%s client start: id cannot be empty", name)
	} else if s.Address == "" {
		return fmt.Errorf("%s client start: address cannot be empty", name)
	}

	if s.Log == nil {
		s.Log = defaultLog
	}
	if len(s.ServerOptions) == 0 {
		s.ServerOptions = []grpc.ServerOption{grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: defaultMaxConnectionIdle})}
	}
	if s.Proto == "" {
		s.Proto = defaultProto
	}
	s.jitter = &jitter.Buffer{}

	s.Log.InfoContext(ctx, "starting", "name", name, "address", s.Address)

	if s.Server == nil {
		s.Server = grpc.NewServer(s.ServerOptions...)
		pollpb.RegisterPollServiceServer(s.Server, s)
		reflection.Register(s.Server)
		defer s.Server.Stop()
	}

	listener, err := net.Listen(s.Proto, s.Address)
	if err != nil {
		return err
	}
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		if err := s.Server.Serve(listener); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.Log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		s.Log.ErrorContext(ctx, "server failed", "name", name, "err", err)
		return err
	}
}
