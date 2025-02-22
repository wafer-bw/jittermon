package sampler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/traceroute"
)

var _ graceful.Runner = (*TraceRoute)(nil)

type TraceRoute struct {
	id       string
	addr     string
	maxHops  int
	timeout  time.Duration
	recorder rec.Recorder
	ticker   *time.Ticker
	log      *slog.Logger
}

type TraceRouteOptions struct {
	ID       string
	Address  string
	MaxHops  int
	Timeout  time.Duration
	Interval time.Duration
	Recorder rec.Recorder
	Log      *slog.Logger
}

func NewTraceRoute(options TraceRouteOptions) (TraceRoute, error) {
	if options.Interval == 0 {
		return TraceRoute{}, fmt.Errorf("sampler.NewTraceRoute: interval is required")
	}

	tr := TraceRoute{
		id:       options.ID,
		addr:     options.Address,
		maxHops:  options.MaxHops,
		timeout:  options.Timeout,
		recorder: options.Recorder,
		ticker:   time.NewTicker(options.Interval),
		log:      slog.New(slog.DiscardHandler),
	}

	if options.Log != nil {
		tr.log = options.Log
	}

	return tr, nil
}

func (tr TraceRoute) Start(ctx context.Context) error {
	defer tr.ticker.Stop()

	for {
		select {
		case <-tr.ticker.C:
			if err := tr.sample(ctx); err != nil {
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

func (tr TraceRoute) sample(ctx context.Context) error {
	start := time.Now()

	opts := traceroute.TraceOptions{MaxHops: tr.maxHops, Timeout: tr.timeout}
	hops, err := traceroute.Trace(tr.addr, opts)
	if err != nil {
		tr.log.Error("failed traceroute", "err", err)
		return fmt.Errorf("sampler.TraceRoute.Trace failed traceroute: %w", err)
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
