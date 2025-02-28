package p2platency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/durationpb"
)

var _ graceful.Runner = (*Server)(nil)

const (
	serverName        string        = "latency server"
	maxConnectionIdle time.Duration = 5 * time.Minute
	DefaultProtocol   string        = "tcp"
)

var defaultServerGRPCOpts []grpc.ServerOption = []grpc.ServerOption{
	grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: maxConnectionIdle}),
}

type ServerOption func(*Server) error

func ServerID(id string) ServerOption {
	return func(c *Server) error {
		c.id = id
		return nil
	}
}

func ServerRecorder(recorder rec.Recorder) ServerOption {
	return func(c *Server) error {
		c.recorder = recorder
		return nil
	}
}

func ServerLog(log *slog.Logger) ServerOption {
	return func(c *Server) error {
		c.log = log
		return nil
	}
}

func ServerGRPCOptions(opts ...grpc.ServerOption) ServerOption {
	return func(c *Server) error {
		c.grpcOpts = opts
		return nil
	}
}

func ServerEnableReflection() ServerOption {
	return func(c *Server) error {
		c.reflectionEnabled = true
		return nil
	}
}

func ServerProtocol(proto string) ServerOption {
	return func(c *Server) error {
		c.proto = proto
		return nil
	}
}

// TODO: ServerEnv() ServerOption

type Server struct {
	id                string
	addr              string
	proto             string
	server            *grpc.Server
	grpcOpts          []grpc.ServerOption
	reflectionEnabled bool
	requestBuffers    jitter.HostPacketBuffers // TODO: accept interface.
	recorder          rec.Recorder
	log               *slog.Logger

	pollpb.UnimplementedPollServiceServer
}

func NewServer(addr string, opts ...ServerOption) (*Server, error) {
	s := &Server{
		addr:           addr,
		proto:          DefaultProtocol,
		requestBuffers: jitter.NewHostPacketBuffers(),
		recorder:       rec.NoOp,
		grpcOpts:       defaultServerGRPCOpts,
		log:            defaultLogger,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	// TODO: verify this doesn't mutate the original logger
	s.log = s.log.With(
		"id", s.id,
		"name", serverName,
		"addr", s.addr,
	)

	s.server = grpc.NewServer(s.grpcOpts...) // TODO: create in Start.

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

	s.requestBuffers.Sample(srcID, jitter.Packet{S: sentAt, R: now})
	jitter, ok := s.requestBuffers.Jitter(srcID)
	if !ok {
		return resp, nil
	}
	resp.SetJitter(durationpb.New(jitter))

	labels := rec.Labels{{K: "src", V: srcID}, {K: "dst", V: s.id}}
	s.recorder.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Val: jitter, Labels: labels})

	return resp, nil
}

func (s Server) Start(ctx context.Context) error {
	s.log.Info("starting")

	pollpb.RegisterPollServiceServer(s.server, s)
	if s.reflectionEnabled {
		reflection.Register(s.server)
	}

	listener, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	return s.server.Serve(listener)
}

func (s Server) Stop(ctx context.Context) error {
	s.log.Debug("stopping")

	ok := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(ok)
	}()

	select {
	case <-ok:
		// fallthrough
	case <-ctx.Done():
		s.server.Stop()
		return fmt.Errorf("graceful stop of %s (%s) failed: %w", serverName, s.id, ctx.Err())
	}

	s.log.Debug("stopped")
	return nil
}
