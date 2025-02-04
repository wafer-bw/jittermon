package peer

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	Addr    string
	Handler pollpb.PollServiceServer
	Log     *slog.Logger

	server *grpc.Server
}

func (s *Server) Start(ctx context.Context) error {
	server := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      5 * time.Second,
			MaxConnectionAgeGrace: 1 * time.Second,
			MaxConnectionIdle:     5 * time.Second,
		}),
		// grpc.MaxConcurrentStreams(1), // TODO: re-enable.
		// grpc.MaxRecvMsgSize(1024), // TODO: re-enable.
		// grpc.MaxSendMsgSize(1024), // TODO: re-enable.
		grpc.ConnectionTimeout(5*time.Second),
		// grpc.MaxHeaderListSize(1024), // TODO: re-enable.
	)
	s.server = server

	pollpb.RegisterPollServiceServer(server, s.Handler)
	reflection.Register(server)

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	s.Log.Info("listening", "addr", listener.Addr())
	return s.server.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	ok := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(ok)
	}()

	select {
	case <-ok:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		return ctx.Err()
	}
}
