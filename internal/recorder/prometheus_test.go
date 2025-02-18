package recorder_test

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
)

func TestNewPrometheus(t *testing.T) {
	t.Parallel()

	t.Run("returns a new prometheus", func(t *testing.T) {
		t.Parallel()

		p, err := recorder.NewPrometheus(":8080", nil)
		require.NoError(t, err)
		require.NotNil(t, p)
	})
}

func TestPrometheus_DefaultRecorders(t *testing.T) {
	t.Parallel()

	t.Run("returns intended amount of default recorders", func(t *testing.T) {
		t.Parallel()

		p, err := recorder.NewPrometheus(":8080", nil)
		require.NoError(t, err)

		recorders := p.DefaultRecorders()
		require.Len(t, recorders, 2)
	})
}

func TestPrometheus_Start(t *testing.T) {
	t.Parallel()

	t.Run("starts the server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))
		p, err := recorder.NewPrometheus(addr, nil)
		require.NoError(t, err)

		go func() {
			err = p.Start(ctx)
			require.NoError(t, err)
		}()
	})

	t.Run("returns error if unable to start server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		p, err := recorder.NewPrometheus("-1", nil)
		require.NoError(t, err)

		err = p.Start(ctx)
		require.Error(t, err)
	})
}

func TestPrometheus_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stops the server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))
		p, err := recorder.NewPrometheus(addr, nil)
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
}

func TestPrometheus_RecordDuration(t *testing.T) {
	t.Parallel()

	t.Run("records duration", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		src, dst := "a", "b"
		samples := []recorder.Sample{
			{Type: "abc", Src: src, Dst: dst, Val: 1 * time.Second},
			{Type: "def", Src: src, Dst: dst, Val: 1 * time.Second},
		}

		p, err := recorder.NewPrometheus(":8080", nil)
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordDuration(noop).Record(ctx, samples[0])
		p.RecordDuration(noop).Record(ctx, samples[1])

		require.Len(t, p.GetHistograms(), 2)
	})

	t.Run("does not record non-duration values", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		src, dst := "a", "b"
		samples := []recorder.Sample{
			{Type: "abc", Src: src, Dst: dst, Val: "abc"},
			{Type: "def", Src: src, Dst: dst, Val: struct{}{}},
		}

		p, err := recorder.NewPrometheus(":8080", nil)
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
		src, dst := "a", "b"
		samples := []recorder.Sample{
			{Type: "abc", Src: src, Dst: dst, Val: struct{}{}},
			{Type: "def", Src: src, Dst: dst, Val: struct{}{}},
		}

		p, err := recorder.NewPrometheus(":8080", nil)
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordIncrement(noop).Record(ctx, samples[0])
		p.RecordIncrement(noop).Record(ctx, samples[1])

		require.Len(t, p.GetCounters(), 2)
	})

	t.Run("does not record non-increment values", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		src, dst := "a", "b"
		samples := []recorder.Sample{
			{Type: "abc", Src: src, Dst: dst, Val: 1},
			{Type: "def", Src: src, Dst: dst, Val: 1 * time.Second},
		}

		p, err := recorder.NewPrometheus(":8080", nil)
		require.NoError(t, err)

		noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})
		p.RecordIncrement(noop).Record(ctx, samples[0])
		p.RecordIncrement(noop).Record(ctx, samples[1])

		require.Len(t, p.GetCounters(), 0)
	})
}
