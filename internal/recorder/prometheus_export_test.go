package recorder

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

// export for testing.
func (r *Prometheus) GetLogger() *slog.Logger {
	return r.log
}

// export for testing.
func (r *Prometheus) GetHistograms() map[string]*prometheus.HistogramVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.histograms
}

// export for testing.
func (r *Prometheus) GetCounters() map[string]*prometheus.CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counters
}
