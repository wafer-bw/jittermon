package traceroute

import (
	"context"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

type Tracer interface {
	Trace(ctx context.Context, dst string) (Hops, error)
}

type Recorder interface {
	recorder.Recorder
}
