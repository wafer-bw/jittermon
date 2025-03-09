package traceroute_test

//go:generate go run go.uber.org/mock/mockgen -source=traceroute.go -destination=traceroute_mocks_test.go -package=traceroute_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/traceroute"
	"go.uber.org/mock/gomock"
)

func TestNewTraceRoute(t *testing.T) {
	t.Parallel()

	t.Run("successfully creates new populated trace route", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		log := new(slog.Logger)

		recorderCalled := false
		rec := recorder.RecorderFunc(func(_ context.Context, _ recorder.Sample) {
			recorderCalled = true
		})

		tr, err := traceroute.NewTraceRoute(
			traceroute.WithID(" id "),
			traceroute.WithAddress("localhost:8080"),
			traceroute.WithInterval(250*time.Millisecond),
			traceroute.WithMaxHops(24),
			traceroute.WithRecorder(rec),
			traceroute.WithLog(log),
		)
		require.NoError(t, err)
		require.NotNil(t, tr)

		r := tr.GetRecorder()
		r.Record(ctx, recorder.Sample{})
		require.True(t, recorderCalled)

		require.Equal(t, "id", tr.GetID())
		require.Equal(t, "localhost:8080", tr.GetAddress())
		require.Equal(t, 250*time.Millisecond, tr.GetInterval())
		require.Equal(t, 24, tr.GetMaxHops())
	})

	t.Run("successfully creates new trace route with default values", func(t *testing.T) {
		t.Parallel()

		tr, err := traceroute.NewTraceRoute()
		require.NoError(t, err)
		require.NotNil(t, tr)
	})

	t.Run("successfully creates new trace route when given blank id", func(t *testing.T) {
		t.Parallel()

		tr, err := traceroute.NewTraceRoute(traceroute.WithID(""))
		require.NoError(t, err)
		require.NotNil(t, tr)
		require.NotEmpty(t, tr.GetID())
	})

	t.Run("successfully creates new trace route when given blank address", func(t *testing.T) {
		t.Parallel()

		tr, err := traceroute.NewTraceRoute(traceroute.WithAddress(""))
		require.NoError(t, err)
		require.NotNil(t, tr)
		require.Equal(t, traceroute.DefaultAddress, tr.GetAddress())
	})

	t.Run("successfully creates new trace route when given zeroed interval", func(t *testing.T) {
		t.Parallel()

		tr, err := traceroute.NewTraceRoute(traceroute.WithInterval(0))
		require.NoError(t, err)
		require.NotNil(t, tr)
		require.Equal(t, traceroute.DefaultInterval, tr.GetInterval())
	})

	t.Run("successfully creates new trace route when given zeroed max hops", func(t *testing.T) {
		t.Parallel()

		tr, err := traceroute.NewTraceRoute(traceroute.WithMaxHops(0))
		require.NoError(t, err)
		require.NotNil(t, tr)
		require.Equal(t, traceroute.DefaultMaxHops, tr.GetMaxHops())
	})

	t.Run("does not panic when passed nil options", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() {
			_, err := traceroute.NewTraceRoute(nil, nil)
			require.NoError(t, err)
		})
	})

	t.Run("executes provided options", func(t *testing.T) {
		t.Parallel()

		var called bool
		optOk := func(p *traceroute.TraceRoute) error {
			called = true
			return nil
		}

		optFail := func(p *traceroute.TraceRoute) error {
			return errors.New("error")
		}

		_, err := traceroute.NewTraceRoute(optOk)
		require.NoError(t, err)
		require.True(t, called)

		_, err = traceroute.NewTraceRoute(optFail)
		require.Error(t, err)
	})
}

func TestTraceRoute_Trace(t *testing.T) {
	t.Parallel()

	t.Run("successfully traces route", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockTracer := NewMockTracer(gomock.NewController(t))
		mockRecorder := NewMockRecorder(gomock.NewController(t))
		hops := traceroute.Hops{
			{Addr: "192.168.1.1", Name: "192.168.1.1", Hop: 1, RTT: ptr(2 * time.Millisecond)},
			{Addr: "1.1.1.1", Name: "something.example.com", Hop: 2, RTT: ptr(4 * time.Millisecond)},
		}

		tr, err := traceroute.NewTraceRoute(
			traceroute.WithInterval(10*time.Millisecond),
			traceroute.WithRecorder(mockRecorder),
		)
		require.NoError(t, err)

		tr.SetTracer(mockTracer)

		mockTracer.EXPECT().Trace(gomock.Any(), tr.GetAddress()).Return(hops, nil).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(_ context.Context, sample recorder.Sample) {
			require.Equal(t, recorder.SampleTypeHopRTT, sample.Type)
			v, ok := sample.Val.(*time.Duration)
			require.True(t, ok)
			require.NotNil(t, v)
			require.Equal(t, 2*time.Millisecond, *v)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(_ context.Context, sample recorder.Sample) {
			require.Equal(t, recorder.SampleTypeHopRTT, sample.Type)
			v, ok := sample.Val.(*time.Duration)
			require.True(t, ok)
			require.NotNil(t, v)
			require.Equal(t, 4*time.Millisecond, *v)
		}).Times(1)

		err = tr.Trace(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when underlying tracer fails", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockTracer := NewMockTracer(gomock.NewController(t))
		mockRecorder := NewMockRecorder(gomock.NewController(t))

		tr, err := traceroute.NewTraceRoute(
			traceroute.WithInterval(10*time.Millisecond),
			traceroute.WithRecorder(mockRecorder),
		)
		require.NoError(t, err)

		tr.SetTracer(mockTracer)

		mockTracer.EXPECT().Trace(gomock.Any(), tr.GetAddress()).Return(nil, errors.New("error")).Times(1)

		err = tr.Trace(ctx)
		require.Error(t, err)
	})
}

func TestTraceRoute_Start(t *testing.T) {
	t.Parallel()

	t.Run("successfully starts trace route tick loop", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockTracer := NewMockTracer(gomock.NewController(t))
		mockRecorder := NewMockRecorder(gomock.NewController(t))

		tr, err := traceroute.NewTraceRoute(
			traceroute.WithInterval(10*time.Millisecond),
			traceroute.WithRecorder(mockRecorder),
		)
		require.NoError(t, err)

		tr.SetTracer(mockTracer)

		mockTracer.EXPECT().Trace(gomock.Any(), tr.GetAddress()).DoAndReturn(func(ctx context.Context, _ string) (traceroute.Hops, error) {
			return nil, errors.New("error")
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

		err = tr.Start(ctx)
		require.Error(t, err)
		require.Equal(t, "error", err.Error())
	})

	t.Run("exits when stop channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		tr, err := traceroute.NewTraceRoute()
		require.NoError(t, err)

		close(tr.GetStopCh())

		err = tr.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		tr, err := traceroute.NewTraceRoute()
		require.NoError(t, err)

		err = tr.Start(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestTraceRoute_Stop(t *testing.T) {
	t.Parallel()

	t.Run("successful stop when stopped channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		tr, err := traceroute.NewTraceRoute()
		require.NoError(t, err)

		close(tr.GetStoppedCh())

		err = tr.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		tr, err := traceroute.NewTraceRoute()
		require.NoError(t, err)

		err = tr.Stop(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}
