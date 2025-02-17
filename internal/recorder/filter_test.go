package recorder_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

func TestMetricFilter(t *testing.T) {
	t.Parallel()

	t.Run("filters recorder calls by sample type", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		f := recorder.MetricFilter(recorder.SampleType("bar"))
		r := f(recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
			require.NotEqual(t, recorder.SampleType("foo"), s.Type)
			require.Equal(t, recorder.SampleType("bar"), s.Type)
			// require.Equal(t, recorder.SampleType("bar"), s.Type)
		}))

		r.Record(ctx, recorder.Sample{Type: "foo"})
		r.Record(ctx, recorder.Sample{Type: "bar"})
	})
}
