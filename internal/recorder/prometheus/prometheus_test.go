package prometheus_test

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/go-toolbox/always"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/recorder/prometheus"
)

func TestNewPrometheus(t *testing.T) {
	t.Parallel()

	t.Run("returns a new prometheus", func(t *testing.T) {
		t.Parallel()

		p, err := prometheus.New(":8080", nil)
		require.NoError(t, err)
		require.NotNil(t, p)
	})
}

func TestPrometheus_DefaultRecorders(t *testing.T) {
	t.Parallel()

	t.Run("returns intended amount of default recorders", func(t *testing.T) {
		t.Parallel()

		p, err := prometheus.New(":8080", nil)
		require.NoError(t, err)

		recorders := p.DefaultRecorders()
		require.Len(t, recorders, 2)
	})
}

func TestPrometheus_StartStop(t *testing.T) {
	t.Parallel()

	t.Run("starts & stops the server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))
		p, err := prometheus.New(addr, nil)
		require.NoError(t, err)

		go func() {
			err := p.Start(ctx)
			require.ErrorIs(t, err, http.ErrServerClosed)
		}()

		require.Eventually(t, func() bool {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return false
			}
			conn.Close()
			return true
		}, 1*time.Second, 25*time.Millisecond)

		err = p.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("returns error if unable to start server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		p, err := prometheus.New("-1", nil)
		require.NoError(t, err)

		err = p.Start(ctx)
		require.Error(t, err)
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

		p, err := prometheus.New(":8080", nil)
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

		p, err := prometheus.New(":8080", nil)
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

		p, err := prometheus.New(":8080", nil)
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

		p, err := prometheus.New(":8080", nil)
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordIncrement(noop).Record(ctx, samples[0])
		p.RecordIncrement(noop).Record(ctx, samples[1])

		require.Len(t, p.GetCounters(), 0)
	})
}
