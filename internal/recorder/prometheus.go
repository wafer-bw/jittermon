package recorder

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/jittermon/internal/peer"
)

const (
	namespace    string        = "jittermon"
	readTimeout  time.Duration = 1 * time.Second
	writeTimeout time.Duration = 2 * time.Second
	idleTimeout  time.Duration = 5 * time.Second
)

var _ peer.Recorder = (*Prometheus)(nil)

type Prometheus struct {
	mu         *sync.Mutex
	addr       string
	server     *http.Server
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

func NewPrometheus(addr string) *Prometheus {
	return &Prometheus{
		mu:   &sync.Mutex{},
		addr: addr,
	}
}

func (r *Prometheus) Record(tsm time.Time, key, src, dst string, dur *time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.histograms == nil {
		r.histograms = map[string]*prometheus.HistogramVec{}
	}
	if r.counters == nil {
		r.counters = map[string]*prometheus.CounterVec{}
	}

	hist, ok := r.histograms[key]
	if !ok {
		hist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s_duration_seconds", key),
			Help:      fmt.Sprintf("A histogram of %s durations", key),
			Buckets:   []float64{0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.5, 1},
		}, []string{"src", "dst"}) // TODO: change to local/remote?
		if err := prometheus.Register(hist); err != nil {
			return fmt.Errorf("could not register histogram for %s: %w", key, err)
		}
		r.histograms[key] = hist
	}

	count, ok := r.counters[key]
	if !ok {
		count = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s_requests_total", key),
			Help:      fmt.Sprintf("Total number of %s observations", key),
		}, []string{"src", "dst"}) // TODO: change to local/remote?
		if err := prometheus.Register(count); err != nil {
			return fmt.Errorf("could not register counter for %s: %w", key, err)
		}
		r.counters[key] = count
	}

	if dur != nil {
		hist.WithLabelValues(src, dst).Observe(dur.Seconds())
	}
	count.WithLabelValues(src, dst).Inc()

	return nil
}

func (r *Prometheus) Start(ctx context.Context) error {
	s := &http.Server{
		Addr:         r.addr,
		Handler:      promhttp.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	r.server = s

	return s.ListenAndServe()
}

func (r *Prometheus) Stop(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}
