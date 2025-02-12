package recorder

import (
	"context"

	"github.com/wafer-bw/jittermon/internal/peer"
)

func MetricFilter(metrics ...peer.MetricType) func(next peer.Recorder) peer.Recorder {
	allowList := map[string]struct{}{}
	for _, metric := range metrics {
		allowList[string(metric)] = struct{}{}
	}

	return func(next peer.Recorder) peer.Recorder {
		return peer.RecorderFunc(func(ctx context.Context, s peer.MetricSample) {
			if _, ok := allowList[string(s.Type)]; ok {
				next.Record(ctx, s)
				return
			}
		})
	}
}
