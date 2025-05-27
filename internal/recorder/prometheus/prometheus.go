package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	namespace    string        = "jittermon"     // TODO: make controllable.
	readTimeout  time.Duration = 1 * time.Second // TODO: make controllable.
	writeTimeout time.Duration = 2 * time.Second // TODO: make controllable.
	idleTimeout  time.Duration = 5 * time.Second // TODO: make controllable.
)

type Prometheus struct {
	log        *slog.Logger
	mu         *sync.Mutex
	server     *http.Server
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

// New returns a new [Prometheus] which must be started and stopped using
// [Prometheus.Start] and [Prometheus.Stop] respectively.
func New(addr string, log *slog.Logger) (*Prometheus, error) {
	r := &Prometheus{
		mu:  &sync.Mutex{},
		log: log,
	}

	if log == nil {
		r.log = slog.New(slog.DiscardHandler)
	}

	r.server = &http.Server{
		Addr:         addr,
		Handler:      promhttp.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

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

// Start the prometheus metrics endpoint server.
func (r *Prometheus) Start(ctx context.Context) error {
	r.log.Info("starting prometheus server", "addr", r.server.Addr)
	return r.server.ListenAndServe()
}

// Stop the prometheus metrics endpoint server.
func (r *Prometheus) Stop(ctx context.Context) error {
	r.log.Debug("stopping prometheus server")
	return r.server.Shutdown(ctx)
}
