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
	addr    string
	handler pollpb.PollServiceServer
	log     *slog.Logger
	server  *grpc.Server
}

func NewServer(addr string, handler pollpb.PollServiceServer, log *slog.Logger) (*Server, error) {
	s := &Server{
		addr:    addr,
		handler: handler,
		log:     log,
	}

	if log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}

	s.server = grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      maxConnectionAge,
			MaxConnectionAgeGrace: maxConnectionAgeGrace,
			MaxConnectionIdle:     maxConnectionIdle,
		}),
	)

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	pollpb.RegisterPollServiceServer(s.server, s.handler)
	reflection.Register(s.server)

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	s.log.Info("starting gRPC server", "addr", listener.Addr())

	return s.server.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.log.Info("stopping server", "addr", s.addr)

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
	addr     string
	poller   DoPoller
	interval time.Duration
	log      *slog.Logger
	doneCh   chan struct{}
	stopCh   chan struct{}
}

func NewClient(addr string, poller DoPoller, interval time.Duration, log *slog.Logger) (*Client, error) {
	c := &Client{
		addr:     addr,
		poller:   poller,
		interval: interval,
		log:      log,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	if log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}

	return c, nil
}

func (c *Client) Start(ctx context.Context) error {
	defer close(c.doneCh)
	c.log.Info("starting client", "addr", c.addr)

	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pollpb.NewPollServiceClient(conn)

	t := time.NewTicker(c.interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			pollCtx, cancel := context.WithTimeout(ctx, c.interval)
			_ = c.poller.DoPoll(pollCtx, client, c.addr)
			cancel()
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.log.Debug("stopping client", "addr", c.addr)

	close(c.stopCh)

	select {
	case <-c.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop failed: %w", ctx.Err())
	}
}
