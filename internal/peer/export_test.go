package peer

import (
	"log/slog"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

// export for testing.
func (p Peer) GetLogger() *slog.Logger {
	return p.log
}

// export for testing.
func (p Peer) GetRecorder() recorder.Recorder {
	return p.r
}

// export for testing.
func (p Peer) GetID() string {
	return p.id
}
