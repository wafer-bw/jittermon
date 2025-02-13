package comms

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

const (
	maxConnectionAge      time.Duration = 10 * time.Second
	maxConnectionIdle     time.Duration = 2 * time.Second
	maxConnectionAgeGrace time.Duration = 2 * time.Second
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
			MaxConnectionAge:      maxConnectionAge,
			MaxConnectionAgeGrace: maxConnectionAgeGrace,
			MaxConnectionIdle:     maxConnectionIdle,
		}),
	)
	s.server = server

	pollpb.RegisterPollServiceServer(server, s.Handler)
	reflection.Register(server)

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	s.Log.Info("starting gRPC server", "addr", listener.Addr())

	return s.server.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.Log.Info("stopping server", "addr", s.Addr)

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

type DoPoller interface {
	DoPoll(ctx context.Context, client pollpb.PollServiceClient, addr string) error
}

type Client struct {
	Addr     string
	Poller   DoPoller
	Interval time.Duration
	Log      *slog.Logger

	doneCh chan struct{}
	stopCh chan struct{}
}

func (c *Client) Start(ctx context.Context) error {
	c.Log.Info("starting client", "addr", c.Addr)

	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})
	defer close(c.doneCh)

	conn, err := grpc.NewClient(c.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pollpb.NewPollServiceClient(conn)

	t := time.NewTicker(c.Interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			pollCtx, cancel := context.WithTimeout(ctx, c.Interval)
			_ = c.Poller.DoPoll(pollCtx, client, c.Addr)
			cancel()
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.Log.Debug("stopping client", "addr", c.Addr)

	close(c.stopCh)

	select {
	case <-c.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop failed: %w", ctx.Err())
	}
}
