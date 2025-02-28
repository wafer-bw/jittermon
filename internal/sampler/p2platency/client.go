package p2platency

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
)

type Recorder interface {
	recorder.Recorder
}

type Poller interface {
	Poll(context.Context, PollRequest) (PollResponse, error)
	Start() error
	Stop() error
	Address() string
}

type PollRequest struct {
	ID        string
	Timestamp time.Time
}

type PollResponse struct {
	ID     string
	Jitter *time.Duration
}

var _ graceful.Runner = (*Client)(nil)

const (
	clientName      string        = "latency client"
	DefaultInterval time.Duration = 1 * time.Second
)

var (
	defaultLogger *slog.Logger = slog.New(slog.DiscardHandler)
)

type ClientOption func(*Client) error

func ClientID(id string) ClientOption {
	return func(c *Client) error {
		if id == "" {
			return nil
		}
		c.id = strings.TrimSpace(id)
		return nil
	}
}

func ClientInterval(interval time.Duration) ClientOption {
	return func(c *Client) error {
		if interval <= 0 {
			return nil
		}
		c.interval = interval
		return nil
	}
}

func ClientRecorder(recorder rec.Recorder) ClientOption {
	return func(c *Client) error {
		c.recorder = recorder
		return nil
	}
}

func ClientLog(log *slog.Logger) ClientOption {
	return func(c *Client) error {
		c.log = log
		return nil
	}
}

// TODO: ClientEnv() ClientOption

type Client struct {
	id       string
	interval time.Duration
	poller   Poller
	recorder Recorder
	log      *slog.Logger
	ticker   *time.Ticker
	stopCh   chan struct{} // TODO: is this still needed?
	doneCh   chan struct{} // TODO: is this still needed?
}

func NewClient(poller Poller, options ...ClientOption) (*Client, error) {
	c := &Client{
		id:       littleid.New(),
		poller:   poller,
		interval: DefaultInterval,
		recorder: rec.NoOp,
		log:      defaultLogger,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	c.log = c.log.With(
		"id", c.id,
		"name", clientName,
		"addr", c.poller.Address(),
	)

	return c, nil
}

func (c *Client) sample(ctx context.Context) error {
	start := time.Now()
	req := PollRequest{ID: c.id, Timestamp: start}
	packetLabels := rec.Labels{{K: "src", V: c.id}, {K: "dst", V: c.poller.Address()}}

	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: packetLabels})
	rsp, err := c.poller.Poll(ctx, req)
	if err != nil {
		c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: packetLabels})
		c.log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(start)

	if rsp.ID == "" {
		c.log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	if rsp.Jitter == nil {
		c.log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}

	latencyLabels := rec.Labels{{K: "src", V: c.id}, {K: "dst", V: rsp.ID}}
	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeUpstreamJitter, Val: rsp.Jitter, Labels: latencyLabels})
	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: latencyLabels})

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	c.log.Info("starting")
	defer close(c.doneCh)

	if err := c.poller.Start(); err != nil {
		return err
	}

	c.ticker = time.NewTicker(c.interval)
	defer c.ticker.Stop()

	for {
		select {
		case <-c.ticker.C:
			if err := c.sample(ctx); err != nil {
				c.log.Warn("sample failed", "err", err)
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
	c.log.Debug("stopping")
	defer c.poller.Stop()

	close(c.stopCh)

	select {
	case <-c.doneCh:
		// fallthrough
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s (%s) failed: %w", clientName, c.id, ctx.Err())
	}

	c.log.Debug("stopped")
	return nil
}
