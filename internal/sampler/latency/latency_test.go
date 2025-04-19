package latency_test

//go:generate go run go.uber.org/mock/mockgen -source=latency.go -destination=latency_mocks_test.go -package=latency_test

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/jitter"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/latency"
	"go.uber.org/mock/gomock"
)

func TestClient_Poll(t *testing.T) {
	t.Parallel()

	t.Run("successful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockRecorder := NewMockRecorder(gomock.NewController(t))
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		client := &latency.Client{
			ID:             "client",
			Address:        addr,
			Interval:       1 * time.Second,
			Recorder:       mockRecorder,
			Log:            slog.New(slog.DiscardHandler),
			RequestBuffers: &jitter.Buffer{},
		}
		readyCh := make(chan struct{})
		doneCh := make(chan struct{})
		defer close(doneCh)

		go func() {
			conn, err := net.ListenPacket("udp", addr)
			require.NoError(t, err)
			defer conn.Close()

			close(readyCh)
			for {
				select {
				case <-doneCh:
					return
				default:
					// fall through
				}

				buf := make([]byte, 1024)
				_, clientAddr, err := conn.ReadFrom(buf)
				if err != nil {
					return
				}
				_, err = conn.WriteTo([]byte("ok"), clientAddr)
				require.NoError(t, err)
			}
		}()

		<-readyCh

		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeSentPackets, sample.Type)
			require.Equal(t, struct{}{}, sample.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeRTT, sample.Type)
			require.NotZero(t, sample.Val)
		}).Times(1)

		err = client.Poll(ctx)
		require.NoError(t, err)

		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeSentPackets, sample.Type)
			require.Equal(t, struct{}{}, sample.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeRTT, sample.Type)
			require.NotZero(t, sample.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeRTTJitter, sample.Type)
			require.NotZero(t, sample.Val)
		}).Times(1)

		err = client.Poll(ctx)
		require.NoError(t, err)
	})

	t.Run("failed poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockRecorder := NewMockRecorder(gomock.NewController(t))
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		client := &latency.Client{
			ID:             "client",
			Address:        addr,
			Interval:       1 * time.Second,
			Recorder:       mockRecorder,
			Log:            slog.New(slog.DiscardHandler),
			RequestBuffers: &jitter.Buffer{},
		}

		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeSentPackets, sample.Type)
			require.Equal(t, struct{}{}, sample.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample rec.Sample) {
			require.Equal(t, rec.SampleTypeLostPackets, sample.Type)
			require.Equal(t, struct{}{}, sample.Val)
		}).Times(1)

		err = client.Poll(ctx)
		require.Error(t, err)
	})
}
