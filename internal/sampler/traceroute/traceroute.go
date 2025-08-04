package traceroute

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/littleid"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
)

var _ graceful.Runner = (*TraceRoute)(nil)

const (
	SamplerName     string        = "traceroute"
	DefaultInterval time.Duration = 1 * time.Second
	DefaultAddress  string        = "8.8.8.8"
	DefaultMaxHops  int           = 24
)

type Tracer interface {
	Trace(ctx context.Context, dst string) (Hops, error)
}

type Recorder interface {
	rec.Recorder
}

type TraceRoute struct {
	id        string
	address   string
	maxHops   int
	interval  time.Duration
	tracer    Tracer
	recorder  Recorder
	log       *slog.Logger
	stoppedCh chan struct{}
	stopCh    chan struct{}
}

type Option func(*TraceRoute) error

func WithID(id string) Option {
	return func(tr *TraceRoute) error {
		if id == "" {
			return nil
		}
		tr.id = strings.TrimSpace(id)
		return nil
	}
}

func WithAddress(address string) Option {
	return func(tr *TraceRoute) error {
		if address == "" {
			return nil
		}
		tr.address = address
		return nil
	}
}

func WithInterval(interval time.Duration) Option {
	return func(tr *TraceRoute) error {
		if interval <= 0 {
			return nil
		}
		tr.interval = interval
		return nil
	}
}

func WithMaxHops(maxHops int) Option {
	return func(tr *TraceRoute) error {
		if maxHops <= 0 {
			return nil
		}
		tr.maxHops = maxHops
		return nil
	}
}

func WithRecorder(recorder rec.Recorder) Option {
	return func(tr *TraceRoute) error {
		tr.recorder = recorder
		return nil
	}
}

func WithLog(log *slog.Logger) Option {
	return func(tr *TraceRoute) error {
		tr.log = log
		return nil
	}
}

func NewTraceRoute(options ...Option) (*TraceRoute, error) {
	tr := &TraceRoute{
		id:        littleid.New(),
		address:   DefaultAddress,
		maxHops:   DefaultMaxHops,
		interval:  DefaultInterval,
		recorder:  rec.NoOp,
		log:       slog.New(slog.DiscardHandler),
		stoppedCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(tr); err != nil {
			return nil, err
		}
	}

	tr.tracer = &execTracer{Timeout: tr.interval, MaxHops: tr.maxHops}

	return tr, nil
}

func (tr TraceRoute) Start(ctx context.Context) error {
	tr.log = tr.log.With("id", tr.id, "name", SamplerName, "addr", tr.address)
	tr.log.Info("starting")

	defer close(tr.stoppedCh)

	ticker := time.NewTicker(tr.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := tr.Trace(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-tr.stopCh:
			return nil
		}
	}
}

func (tr TraceRoute) Stop(ctx context.Context) error {
	tr.log.Debug("stopping")

	close(tr.stopCh)

	select {
	case <-tr.stoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", SamplerName, tr.id, ctx.Err())
	}
}

func (tr TraceRoute) Trace(ctx context.Context) error {
	start := time.Now()

	hops, err := tr.tracer.Trace(ctx, tr.address)
	if err != nil {
		return err
	}

	for _, hop := range hops {
		labels := rec.Labels{
			{K: "src", V: tr.id},
			{K: "dst", V: tr.address},
			{K: "hop", V: strconv.Itoa(hop.Hop)},
			{K: "addr", V: hop.Addr},
			{K: "hostname", V: hop.Name},
		}
		tr.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeHopRTT, Val: hop.RTT, Labels: labels})
	}

	return nil
}
