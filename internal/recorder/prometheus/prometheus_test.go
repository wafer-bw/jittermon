package prometheus_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/recorder/prometheus"
)

func TestNewPrometheus(t *testing.T) {
	t.Parallel()

	t.Run("returns a new prometheus", func(t *testing.T) {
		t.Parallel()

		p, err := prometheus.New(":8080")
		require.NoError(t, err)
		require.NotNil(t, p)
	})
}

func TestPrometheus_DefaultRecorders(t *testing.T) {
	t.Parallel()

	t.Run("returns intended amount of default recorders", func(t *testing.T) {
		t.Parallel()

		p, err := prometheus.New(":8080")
		require.NoError(t, err)

		recorders := p.DefaultRecorders()
		require.Len(t, recorders, 2)
	})
}

func TestPrometheus_RecordDuration(t *testing.T) {
	t.Parallel()

	t.Run("records duration", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		samples := []recorder.Sample{
			{Type: "abc", Val: 1 * time.Second},
			{Type: "def", Val: 1 * time.Second},
		}

		p, err := prometheus.New(":8080")
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordDuration(noop).Record(ctx, samples[0])
		p.RecordDuration(noop).Record(ctx, samples[1])

		require.Len(t, p.GetHistograms(), 2)
	})

	t.Run("does not record non-duration values", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		samples := []recorder.Sample{
			{Type: "abc", Val: "abc"},
			{Type: "def", Val: struct{}{}},
		}

		p, err := prometheus.New(":8080")
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordDuration(noop).Record(ctx, samples[0])
		p.RecordDuration(noop).Record(ctx, samples[1])

		require.Len(t, p.GetHistograms(), 0)
	})
}

func TestPrometheus_RecordIncrement(t *testing.T) {
	t.Parallel()

	t.Run("records increment", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		samples := []recorder.Sample{
			{Type: "abc", Val: struct{}{}},
			{Type: "def", Val: struct{}{}},
		}

		p, err := prometheus.New(":8080")
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordIncrement(noop).Record(ctx, samples[0])
		p.RecordIncrement(noop).Record(ctx, samples[1])

		require.Len(t, p.GetCounters(), 2)
	})

	t.Run("does not record non-increment values", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		samples := []recorder.Sample{
			{Type: "abc", Val: 1},
			{Type: "def", Val: 1 * time.Second},
		}

		p, err := prometheus.New(":8080")
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordIncrement(noop).Record(ctx, samples[0])
		p.RecordIncrement(noop).Record(ctx, samples[1])

		require.Len(t, p.GetCounters(), 0)
	})
}
