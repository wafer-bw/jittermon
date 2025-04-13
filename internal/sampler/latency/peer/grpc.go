package peer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/latency/peer/internal/pollpb"
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

type GRPCClientPoller interface {
	pollpb.PollServiceClient
}

type GRPCClient struct {
	ID            string
	Address       string
	Interval      time.Duration
	Recorder      Recorder
	ClientOptions []grpc.DialOption
	Log           *slog.Logger
	StopCh        chan struct{}
	StoppedCh     chan struct{}
	Client        GRPCClientPoller
}

func (c GRPCClient) Poll(ctx context.Context) error {
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

func (c *GRPCClient) Start(ctx context.Context) error {
	c.Log = c.Log.With("id", c.ID, "name", grpcClientName, "address", c.Address)
	c.Log.Info("starting")

	defer close(c.StoppedCh)

	if c.Client == nil {
		conn, err := grpc.NewClient(c.Address, c.ClientOptions...)
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

func (c *GRPCClient) Stop(ctx context.Context) error {
	c.Log.Debug("stopping")

	close(c.StopCh)

	select {
	case <-c.StoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", grpcClientName, c.ID, ctx.Err())
	}
}

type GRPCServer struct {
	ID                      string
	Address                 string
	Proto                   string
	ServerOptions           []grpc.ServerOption
	ServerReflectionEnabled bool
	Recorder                Recorder
	RequestBuffers          *jitter.Buffer // TODO: accept interface.
	Log                     *slog.Logger
	StartedCh               chan struct{}
	StoppedCh               chan struct{}

	Server *grpc.Server

	pollpb.UnimplementedPollServiceServer
}

func (s GRPCServer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
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

func (s *GRPCServer) Start(ctx context.Context) error {
	s.Log = s.Log.With("id", s.ID, "name", grpcServerName, "address", s.Address)
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

func (s *GRPCServer) Stop(ctx context.Context) error {
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
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", grpcServerName, s.ID, ctx.Err())
	}
}
