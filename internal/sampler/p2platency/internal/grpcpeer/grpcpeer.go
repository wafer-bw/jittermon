package grpcpeer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	grpcClientName string = "grpc_p2platency_client"
	grpcServerName string = "grpc_p2platency_server"
)

type Recorder interface {
	rec.Recorder
}

type Client struct {
	ID            string
	Address       string
	Interval      time.Duration
	Recorder      Recorder
	ClientOptions []grpc.DialOption
	Log           *slog.Logger

	client     pollpb.PollServiceClient
	connection *grpc.ClientConn

	// startedCh signals that everything started by [grpcClient.Start] has
	// started.
	//
	// TODO: check if this is needed, it likely protects against races.
	startedCh chan struct{}

	// stopCh signals that everything started by [grpcClient.Start] should stop.
	stopCh chan struct{}

	// stoppedCh signals that everything started by [grpcClient.Start] has
	// stopped.
	stoppedCh chan struct{}
}

func (c Client) Poll(ctx context.Context) error {
	start := time.Now()
	labels := rec.Labels{{K: "src", V: c.ID}, {K: "dst", V: c.Address}}

	req := &pollpb.PollRequest{}
	req.SetId(c.ID)
	req.SetTimestamp(timestamppb.New(start))

	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	rsp, err := c.client.Poll(ctx, req)
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
	c.Log = c.Log.With("id", c.ID, "name", grpcClientName, "address", c.Address)
	c.Log.Info("starting")

	defer close(c.stoppedCh)

	var err error
	c.connection, err = grpc.NewClient(c.Address, c.ClientOptions...)
	if err != nil {
		return err
	}
	defer c.connection.Close()
	c.client = pollpb.NewPollServiceClient(c.connection)

	close(c.startedCh)

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
		case <-c.stopCh:
			return nil
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.Log.Debug("stopping")

	select {
	case <-c.startedCh:
	case <-c.stoppedCh:
	}

	close(c.stopCh)

	select {
	case <-c.stoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", grpcClientName, c.ID, ctx.Err())
	}
}

type Server struct {
	ID                      string
	Address                 string
	Proto                   string
	ServerOptions           []grpc.ServerOption
	ServerReflectionEnabled bool
	Recorder                Recorder
	RequestBuffers          jitter.HostPacketBuffers // TODO: accept interface.
	Log                     *slog.Logger

	Server   *grpc.Server
	Listener net.Listener

	// StartedCh signals that everything started by [grpcClient.Start] has
	// started.
	//
	// TODO: check if this is needed, it likely protects against races.
	StartedCh chan struct{}

	// StoppedCh signals that everything started by [grpcClient.Start] has
	// stopped.
	StoppedCh chan struct{}

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

	s.RequestBuffers.Sample(srcID, jitter.Packet{S: sentAt, R: now})
	jitter, ok := s.RequestBuffers.Jitter(srcID)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := rec.Labels{{K: "src", V: srcID}, {K: "dst", V: s.ID}}
	s.Recorder.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.Log = s.Log.With("id", s.ID, "name", grpcServerName, "address", s.Address)
	s.Log.Info("starting")

	defer close(s.StoppedCh)

	s.Server = grpc.NewServer(s.ServerOptions...)
	pollpb.RegisterPollServiceServer(s.Server, s)
	if s.ServerReflectionEnabled {
		reflection.Register(s.Server)
	}

	var err error
	s.Listener, err = net.Listen(s.Proto, s.Address)
	if err != nil {
		return err
	}
	close(s.StartedCh)

	return s.Server.Serve(s.Listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.Log.Debug("stopping")

	select {
	case <-s.StartedCh:
	case <-s.StoppedCh:
	}

	gracefullyStoppedCh := make(chan struct{})
	go func() {
		s.Server.GracefulStop()
		close(gracefullyStoppedCh)
	}()

	select {
	case <-gracefullyStoppedCh:
		return nil
	case <-ctx.Done():
		s.Server.Stop()
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", grpcServerName, s.ID, ctx.Err())
	}
}
