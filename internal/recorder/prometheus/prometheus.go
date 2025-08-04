package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	Name string = "http_prometheus_server"

	namespace    string        = "jittermon"     // TODO: make configurable.
	readTimeout  time.Duration = 1 * time.Second // TODO: make configurable.
	writeTimeout time.Duration = 2 * time.Second // TODO: make configurable.
	idleTimeout  time.Duration = 5 * time.Second // TODO: make configurable.
)

var defaultLog = slog.New(slog.DiscardHandler)

type Option func(*Prometheus) error

func WithLog(log *slog.Logger) Option {
	return func(p *Prometheus) error {
		if log == nil {
			return nil
		}
		p.log = log
		return nil
	}
}

func WithID(id string) Option {
	return func(p *Prometheus) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		p.id = id
		return nil
	}
}

type Prometheus struct {
	id         string
	log        *slog.Logger
	mu         *sync.Mutex
	server     *http.Server
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

// New returns a new [Prometheus] which must be started and stopped using
// [Prometheus.Start] and [Prometheus.Stop] respectively.
func New(addr string, options ...Option) (*Prometheus, error) {
	if addr == "" {
		return nil, fmt.Errorf("address must not be empty")
	}

	r := &Prometheus{
		id:  littleid.New(),
		mu:  &sync.Mutex{},
		log: defaultLog,
	}

	for _, option := range options {
		if err := option(r); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	r.server = &http.Server{
		Addr:         addr,
		Handler:      promhttp.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	r.log = r.log.With("name", Name, "id", r.id, "addr", addr)

	return r, nil
}

// DefaultRecorders returns the default, recommended recorders.
func (r *Prometheus) DefaultRecorders() []recorder.ChainLink {
	return []recorder.ChainLink{
		r.RecordDuration,
		r.RecordIncrement,
	}
}

// RecordDuration records samples whose [recorder.Sample.Val] is a [time.Duration].
// It records the duration in seconds to a [prometheus.HistogramVec].
func (r *Prometheus) RecordDuration(next recorder.Recorder) recorder.Recorder {
	return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
		defer next.Record(ctx, s)
		r.mu.Lock()
		defer r.mu.Unlock()

		if r.histograms == nil {
			r.histograms = map[string]*prometheus.HistogramVec{}
		}

		key := string(s.Type)
		val, ok := s.Val.(time.Duration)
		if !ok {
			valP, ok := s.Val.(*time.Duration)
			if !ok {
				return
			}
			if valP == nil {
				valP = new(time.Duration)
			}
			val = *valP
		}

		histogram, ok := r.histograms[key]
		if !ok {
			histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s_duration_seconds", key),
				Help:      fmt.Sprintf("A histogram of '%s' durations in seconds", key),
				Buckets:   []float64{0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.5, 1},
			}, s.Labels.Keys())
			if err := prometheus.Register(histogram); err != nil {
				r.log.Error("could not register histogram", "key", key, "err", err)
			}
			r.histograms[key] = histogram
		}

		histogram.WithLabelValues(s.Labels.Values()...).Observe(val.Seconds())
	})
}

// RecordIncrement records samples whose [recorder.Sample.Val] is `strut{}{}`, such
// samples have no value and are only used to increment a counter.
func (r *Prometheus) RecordIncrement(next recorder.Recorder) recorder.Recorder {
	return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
		defer next.Record(ctx, s)
		r.mu.Lock()
		defer r.mu.Unlock()

		if r.counters == nil {
			r.counters = map[string]*prometheus.CounterVec{}
		}

		key := string(s.Type)
		if _, ok := s.Val.(struct{}); !ok {
			return
		}

		counter, ok := r.counters[key]
		if !ok {
			counter = prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s_total", key),
				Help:      fmt.Sprintf("Total number of '%s' observations", key),
			}, s.Labels.Keys())
			if err := prometheus.Register(counter); err != nil {
				r.log.Error("could not register counter", "key", key, "err", err)
			}
			r.counters[key] = counter
		}

		counter.WithLabelValues(s.Labels.Values()...).Inc()
	})
}

// Run the prometheus metrics endpoint server.
func (r *Prometheus) Run(ctx context.Context) error {
	r.log.InfoContext(ctx, "starting")

	errCh := make(chan error)
	go func() {
		if err := r.server.ListenAndServe(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		r.log.WarnContext(ctx, "context done, stopping", "err", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		r.log.ErrorContext(ctx, "server failed", "err", err)
		return err
	}
}
