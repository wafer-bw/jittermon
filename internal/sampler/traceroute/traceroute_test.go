package traceroute_test

//go:generate go run go.uber.org/mock/mockgen -source=ports.go -destination=ports_mocks_test.go -package=traceroute_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/traceroute"
	"go.uber.org/mock/gomock"
)

func TestTraceRoute_Poll(t *testing.T) {
	t.Parallel()

	t.Run("successfully traces route", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockTracer := NewMockTracer(gomock.NewController(t))
		mockRecorder := NewMockRecorder(gomock.NewController(t))
		addr := "1.1.1.1"
		hops := traceroute.Hops{
			{Addr: "192.168.1.1", Name: "192.168.1.1", Hop: 1, RTT: ptr(2 * time.Millisecond)},
			{Addr: addr, Name: "something.example.com", Hop: 2, RTT: ptr(4 * time.Millisecond)},
		}

		c, err := traceroute.NewTraceRoute(addr, mockRecorder,
			traceroute.WithInterval(10*time.Millisecond),
			traceroute.WithTracer(mockTracer),
		)
		require.NoError(t, err)

		mockTracer.EXPECT().Trace(gomock.Any(), addr).Return(hops, nil).Times(1)
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

		err = c.Poll(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when underlying tracer fails", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := "1.1.1.1"
		mockTracer := NewMockTracer(gomock.NewController(t))
		mockRecorder := NewMockRecorder(gomock.NewController(t))

		tr, err := traceroute.NewTraceRoute(addr, mockRecorder,
			traceroute.WithInterval(10*time.Millisecond),
			traceroute.WithTracer(mockTracer),
		)
		require.NoError(t, err)

		mockTracer.EXPECT().Trace(gomock.Any(), addr).Return(nil, errors.New("error")).Times(1)

		err = tr.Poll(ctx)
		require.Error(t, err)
	})
}
