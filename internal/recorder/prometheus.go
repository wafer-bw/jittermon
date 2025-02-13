package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace    string        = "jittermon"
	readTimeout  time.Duration = 1 * time.Second
	writeTimeout time.Duration = 2 * time.Second
	idleTimeout  time.Duration = 5 * time.Second
)

// TODO: docstring.
type Prometheus struct {
	log        *slog.Logger
	mu         *sync.Mutex
	addr       string
	server     *http.Server
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

// TODO: docstring.
func NewPrometheus(addr string, log *slog.Logger) *Prometheus {
	return &Prometheus{
		mu:   &sync.Mutex{},
		addr: addr,
		log:  log,
	}
}

// TODO: docstring.
func (r Prometheus) DefaultRecorders() []func(Recorder) Recorder {
	return []func(Recorder) Recorder{
		r.RecordDuration,
		r.RecordIncrement,
	}
}

// TODO: docstring.
func (r *Prometheus) RecordDuration(next Recorder) Recorder {
	return RecorderFunc(func(ctx context.Context, s Sample) {
		defer next.Record(ctx, s)
		r.mu.Lock()
		defer r.mu.Unlock()

		if r.histograms == nil {
			r.histograms = map[string]*prometheus.HistogramVec{}
		}

		key := string(s.Type)
		val, ok := s.Val.(time.Duration)
		if !ok {
			return
		}

		histogram, ok := r.histograms[key]
		if !ok {
			histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s_duration_seconds", key),
				Help:      fmt.Sprintf("A histogram of '%s' durations in seconds", key),
				Buckets:   []float64{0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.5, 1},
			}, []string{"src", "dst"}) // TODO: change to local/remote?
			if err := prometheus.Register(histogram); err != nil {
				r.log.Error("could not register histogram", "key", key, "err", err)
			}
			r.histograms[key] = histogram
		}

		histogram.WithLabelValues(s.Src, s.Dst).Observe(val.Seconds())
	})
}

// TODO: docstring.
func (r *Prometheus) RecordIncrement(next Recorder) Recorder {
	return RecorderFunc(func(ctx context.Context, s Sample) {
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
			}, []string{"src", "dst"}) // TODO: change to local/remote?
			if err := prometheus.Register(counter); err != nil {
				r.log.Error("could not register counter", "key", key, "err", err)
			}
			r.counters[key] = counter
		}

		counter.WithLabelValues(s.Src, s.Dst).Inc()
	})
}

// TODO: docstring.
func (r *Prometheus) Start(ctx context.Context) error {
	s := &http.Server{
		Addr:         r.addr,
		Handler:      promhttp.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	r.server = s

	r.log.Info("starting prometheus server", "addr", r.addr)
	return s.ListenAndServe()
}

// TODO: docstring.
func (r *Prometheus) Stop(ctx context.Context) error {
	r.log.Debug("stopping prometheus server")
	return r.server.Shutdown(ctx)
}
