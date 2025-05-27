package logger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

// Recorder returns a [recorder.ChainLink] that logs samples using the provided
// [slog.Logger].
//
// It ignores hop & sent packets.
func Recorder(logger *slog.Logger) func(next recorder.Recorder) recorder.Recorder {
	return func(next recorder.Recorder) recorder.Recorder {
		return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
			defer next.Record(ctx, s)
			if logger == nil {
				return
			}

			duration, durationOk := getDuration(s.Val)
			if durationOk {
				duration = duration.Round(time.Microsecond)
			}

			log := logger
			for _, l := range s.Labels {
				if l.K == "" || l.V == "" {
					continue
				}
				log = log.With(l.K, l.V)
			}

			switch s.Type {
			case recorder.SampleTypeRTT:
				if durationOk {
					log.InfoContext(ctx, fmt.Sprintf("ping: %s", duration))
				}
			case recorder.SampleTypeRTTJitter:
				if durationOk {
					log.InfoContext(ctx, fmt.Sprintf("jitter: %s", duration))
				}
			case recorder.SampleTypeDownstreamJitter:
				if durationOk {
					log.InfoContext(ctx, fmt.Sprintf("downstream jitter: %s", duration))
				}
			case recorder.SampleTypeUpstreamJitter:
				if durationOk {
					log.InfoContext(ctx, fmt.Sprintf("upstream jitter: %s", duration))
				}
			case recorder.SampleTypeLostPackets:
				log.InfoContext(ctx, "lost packet")
			case // ignored:
				recorder.SampleTypeSentPackets,
				recorder.SampleTypeHopRTT:
				return
			}
		})
	}
}

func getDuration(v any) (time.Duration, bool) {
	d, ok := v.(time.Duration)
	if !ok {
		dp, ok := v.(*time.Duration)
		if !ok {
			return 0, false
		} else if dp == nil {
			return 0, false
		}
		d = *dp
	}

	return d, true
}

func clone(l *slog.Logger) *slog.Logger {
	c := *l
	return &c
}
