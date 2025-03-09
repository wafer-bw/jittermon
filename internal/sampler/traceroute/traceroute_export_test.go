package traceroute

import (
	"log/slog"
	"time"
)

// export for testing.
func (tr *TraceRoute) SetTracer(tracer Tracer) {
	tr.tracer = tracer
}

// export for testing.
func (tr TraceRoute) GetRecorder() Recorder {
	return tr.recorder
}

// export for testing.
func (tr TraceRoute) GetLog() *slog.Logger {
	return tr.log
}

// export for testing.
func (tr TraceRoute) GetID() string {
	return tr.id
}

// export for testing.
func (tr TraceRoute) GetAddress() string {
	return tr.address
}

// export for testing.
func (tr TraceRoute) GetInterval() time.Duration {
	return tr.interval
}

// export for testing.
func (tr TraceRoute) GetMaxHops() int {
	return tr.maxHops
}

// export for testing.
func (tr TraceRoute) GetStopCh() chan struct{} {
	return tr.stopCh
}

// export for testing.
func (tr TraceRoute) GetStoppedCh() chan struct{} {
	return tr.stoppedCh
}
