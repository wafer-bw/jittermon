package recorder

import (
	"context"
)

// TODO: docstring.
func MetricFilter(metrics ...SampleType) func(next Recorder) Recorder {
	allowList := map[string]struct{}{}
	for _, metric := range metrics {
		allowList[string(metric)] = struct{}{}
	}

	return func(next Recorder) Recorder {
		return RecorderFunc(func(ctx context.Context, s Sample) {
			if _, ok := allowList[string(s.Type)]; ok {
				next.Record(ctx, s)
				return
			}
		})
	}
}
