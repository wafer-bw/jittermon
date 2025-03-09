package recorder_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

func TestLabels_Keys(t *testing.T) {
	t.Parallel()

	t.Run("returns the keys of the labels", func(t *testing.T) {
		t.Parallel()

		l := recorder.Labels{
			{K: "foo", V: "bar"},
			{K: "baz", V: "qux"},
		}

		keys := l.Keys()

		require.Len(t, keys, 2)
		require.Contains(t, keys, "foo")
		require.Contains(t, keys, "baz")
	})
}

func TestLabels_Values(t *testing.T) {
	t.Parallel()

	t.Run("returns the values of the labels", func(t *testing.T) {
		t.Parallel()

		l := recorder.Labels{
			{K: "foo", V: "bar"},
			{K: "baz", V: "qux"},
		}

		values := l.Values()

		require.Len(t, values, 2)
		require.Contains(t, values, "bar")
		require.Contains(t, values, "qux")
	})
}

func TestChain(t *testing.T) {
	t.Parallel()

	type ctxKey string

	t.Run("returns a Recorder that calls each Recorder in the chain", func(t *testing.T) {
		t.Parallel()

		key := ctxKey("foo")
		ctx := t.Context()
		b1, b2 := new(bool), new(bool)
		ctx = context.WithValue(ctx, key, 0)

		r1 := func(next recorder.Recorder) recorder.Recorder {
			return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
				*b1 = true
				v, ok := ctx.Value(key).(int)
				require.True(t, ok)
				require.Equal(t, 0, v)
				next.Record(context.WithValue(ctx, key, v+1), s)
			})
		}
		r2 := func(next recorder.Recorder) recorder.Recorder {
			return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
				*b2 = true
				v, ok := ctx.Value(key).(int)
				require.True(t, ok)
				require.Equal(t, 1, v)
				next.Record(context.WithValue(ctx, key, v+1), s)
			})
		}

		require.False(t, *b1)
		require.False(t, *b2)

		r := recorder.Chain(r1, r2)
		r.Record(ctx, recorder.Sample{})

		require.True(t, *b1)
		require.True(t, *b2)
	})

	t.Run("returns an noop chain if provided no recorders", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		require.NotPanics(t, func() {
			r := recorder.Chain()
			r.Record(ctx, recorder.Sample{})
		})
	})
}

func TestNoOp(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		recorder.NoOp.Record(t.Context(), recorder.Sample{})
	})
}
