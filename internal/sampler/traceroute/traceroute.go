package traceroute

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
)

var _ graceful.Runner = (*TraceRoute)(nil)

const (
	SamplerName     string        = "traceroute"
	defaultInterval time.Duration = 1 * time.Second
	defaultTimeout  time.Duration = 1 * time.Second
	defaultMaxHops  int           = 24
)

var (
	defaultTracer = &execTracer{Timeout: defaultTimeout, MaxHops: defaultMaxHops}
)

type Tracer interface {
	Trace(ctx context.Context, dst string) (Hops, error)
}

type Recorder interface {
	rec.Recorder
}

type TraceRoute struct {
	id       string
	addr     string
	maxHops  int
	timeout  time.Duration
	interval time.Duration
	tracer   Tracer
	recorder Recorder
	ticker   *time.Ticker
	log      *slog.Logger
}

type TraceRouteOptions struct {
	ID       string
	Address  string
	MaxHops  int
	Timeout  time.Duration
	Interval time.Duration
	Tracer   Tracer
	Recorder Recorder
	Log      *slog.Logger
}

func NewTraceRoute(options TraceRouteOptions) (TraceRoute, error) {
	tr := TraceRoute{
		id:       options.ID,
		addr:     options.Address,
		maxHops:  options.MaxHops,
		timeout:  options.Timeout,
		interval: options.Interval,
		tracer:   options.Tracer,
		recorder: options.Recorder,
		log:      options.Log,
	}

	if tr.tracer == nil {
		tr.tracer = defaultTracer
	}

	if tr.log == nil {
		tr.log = slog.New(slog.DiscardHandler)
	}

	if tr.interval == 0 {
		tr.interval = defaultInterval
	}

	return tr, nil
}

func (tr TraceRoute) Start(ctx context.Context) error {
	ticker := time.NewTicker(tr.interval)
	defer ticker.Stop()

	for {
		select {
		case <-tr.ticker.C:
			if err := tr.Trace(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (tr TraceRoute) Stop(ctx context.Context) error {
	tr.ticker.Stop()
	return nil
}

func (tr TraceRoute) Trace(ctx context.Context) error {
	start := time.Now()

	hops, err := tr.tracer.Trace(ctx, tr.addr)
	if err != nil {
		return err
	}

	for _, hop := range hops {
		labels := rec.Labels{
			{K: "src", V: tr.id},
			{K: "dst", V: tr.addr},
			{K: "hop", V: strconv.Itoa(hop.Hop)},
			{K: "addr", V: hop.Addr},
			{K: "hostname", V: hop.Name},
		}
		tr.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeHopRTT, Val: hop.RTT, Labels: labels})
	}

	return nil
}
