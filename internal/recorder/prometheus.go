package recorder

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wafer-bw/jittermon/internal/peer"
)

const namespace string = "jittermon"

var _ peer.Recorder = (*Prometheus)(nil)

type Prometheus struct {
	Addr string

	server     *http.Server
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

func (r *Prometheus) Record(src, dst peer.PeerID, key string, tsm time.Time, dur *time.Duration) error {
	k := fmt.Sprintf("%s-%s-%s", src, dst, key)

	if r.histograms == nil {
		r.histograms = map[string]*prometheus.HistogramVec{}
	}
	if r.counters == nil {
		r.counters = map[string]*prometheus.CounterVec{}
	}

	hist, ok := r.histograms[k]
	if !ok {
		hist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s_duration_seconds", key),
			Help:      fmt.Sprintf("A histogram of %s durations", key),
			Buckets:   []float64{0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.5, 1},
		}, []string{"src", "dst"}) // TODO: change to local/remote?
		if err := prometheus.Register(hist); err != nil {
			return err
		}
		r.histograms[k] = hist
	}

	count, ok := r.counters[k]
	if !ok {
		count = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s_requests_total", key),
			Help:      fmt.Sprintf("Total number of %s observations", key),
		}, []string{"src", "dst"}) // TODO: change to local/remote?
		if err := prometheus.Register(count); err != nil {
			return err
		}
		r.counters[k] = count
	}

	if dur != nil {
		hist.WithLabelValues(string(src), string(dst)).Observe(dur.Seconds())
	}
	count.WithLabelValues(string(src), string(dst)).Inc()

	return nil
}

func (r *Prometheus) Start(ctx context.Context) error {
	s := &http.Server{
		Addr:         r.Addr,
		Handler:      promhttp.Handler(),
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 2 * time.Second,
		IdleTimeout:  5 * time.Second,
	}
	r.server = s

	return s.ListenAndServe()
}

func (r *Prometheus) Stop(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}
