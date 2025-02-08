package net

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DoPoller interface {
	DoPoll(context.Context, pollpb.PollServiceClient) error
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
			pollCtx, cancel := context.WithTimeout(ctx, c.Interval*5)
			_ = c.Poller.DoPoll(pollCtx, client) // TODO: handle error.
			cancel()
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	close(c.stopCh)

	select {
	case <-c.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop failed: %w", ctx.Err())
	}
}
