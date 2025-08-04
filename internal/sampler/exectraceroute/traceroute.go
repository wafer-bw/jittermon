package exectraceroute

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

type Tracer interface {
	Trace(ctx context.Context, dst string) (Hops, error)
}

type Recorder interface {
	recorder.Recorder
}

const (
	Name string = "exec_traceroute_client"

	defaultMaxHops  int           = 24
	defaultInterval time.Duration = 1 * time.Second
	defaultTimeout  time.Duration = defaultInterval * time.Duration(2)
)

var (
	defaultLog = slog.New(slog.DiscardHandler)
)

type Option func(*TraceRoute) error

func WithID(id string) Option {
	return func(c *TraceRoute) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		c.id = id
		return nil
	}
}

func WithInterval(interval time.Duration) Option {
	return func(c *TraceRoute) error {
		if interval <= 0 {
			return nil
		}
		c.interval = interval
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *TraceRoute) error {
		if timeout <= 0 {
			return nil
		}
		c.timeout = timeout
		return nil
	}
}

func WithMaxHops(maxHops int) Option {
	return func(c *TraceRoute) error {
		if maxHops <= 0 {
			return nil
		}
		c.maxHops = maxHops
		return nil
	}
}

func WithLog(log *slog.Logger) Option {
	return func(c *TraceRoute) error {
		c.log = log
		return nil
	}
}

type TraceRoute struct {
	id       string
	address  string
	maxHops  int
	interval time.Duration
	timeout  time.Duration
	tracer   Tracer
	recorder Recorder
	log      *slog.Logger
}

func NewTraceRoute(address string, recorder Recorder, options ...Option) (*TraceRoute, error) {
	c := &TraceRoute{
		address:  address,
		recorder: recorder,
		id:       littleid.New(),
		maxHops:  defaultMaxHops,
		interval: defaultInterval,
		timeout:  defaultTimeout,
		log:      defaultLog,
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	c.log = c.log.With("name", Name, "id", c.id, "address", c.address)

	return c, nil
}

func (c TraceRoute) Poll(ctx context.Context) error {
	start := time.Now()

	hops, err := c.tracer.Trace(ctx, c.address)
	if err != nil {
		return err
	}

	for _, hop := range hops {
		labels := recorder.Labels{
			{K: "src", V: c.id},
			{K: "dst", V: c.address},
			{K: "hop", V: strconv.Itoa(hop.Hop)},
			{K: "addr", V: hop.Addr},
			{K: "hostname", V: hop.Name},
		}
		c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeHopRTT, Val: hop.RTT, Labels: labels})
	}

	return nil
}

func (c TraceRoute) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.InfoContext(ctx, "starting", "interval", c.interval)

	if c.tracer == nil {
		c.tracer = &execTracer{Timeout: c.timeout, MaxHops: c.maxHops}
	}

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				c.log.WarnContext(ctx, "poll failed", "err", err)
				return err
			}
		case <-ctx.Done():
			c.log.WarnContext(ctx, "context done, stopping", "err", ctx.Err())
			return ctx.Err()
		}
	}
}
