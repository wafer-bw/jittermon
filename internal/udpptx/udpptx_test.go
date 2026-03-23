package udpptx_test

//go:generate go run go.uber.org/mock/mockgen -source=udpptx.go -destination=udpptx_mocks_test.go -package=udpptx_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/udpptx"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/mock/gomock"
)

func TestClient_Poll(t *testing.T) {
	t.Parallel()

	t.Run("successful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		client := udpptx.Client{
			ID:                 "id",
			Address:            addr,
			SentPacketsCounter: mockSentPacketsCounter,
			LostPacketsCounter: mockLostPacketsCounter,
			PingHistogram:      mockPingHistogram,
			JitterHistogram:    mockJitterHistogram,
			Timeout:            25 * time.Millisecond,
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

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, val float64, options ...metric.RecordOption) {
			require.NotZero(t, val)
		}).Times(1)

		err = client.Poll(ctx)
		require.Error(t, err) // expecting an error because there is no jitter the first time around.
		require.Equal(t, "no jitter in response", err.Error())

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, val float64, options ...metric.RecordOption) {
			require.NotZero(t, val)
		}).Times(1)
		mockJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, val float64, options ...metric.RecordOption) {
			require.NotZero(t, val)
		}).Times(1)

		err = client.Poll(ctx)
		require.NoError(t, err)
	})

	t.Run("failed poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		client := udpptx.Client{
			ID:                 "id",
			Address:            addr,
			SentPacketsCounter: mockSentPacketsCounter,
			LostPacketsCounter: mockLostPacketsCounter,
			PingHistogram:      mockPingHistogram,
			JitterHistogram:    mockJitterHistogram,
			Timeout:            25 * time.Millisecond,
		}

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockLostPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)

		err = client.Poll(ctx)
		require.Error(t, err)
	})
}

func TestClient_Start(t *testing.T) {
	t.Parallel()

	t.Run("sucessful start", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		client := udpptx.Client{
			ID:                 "id",
			Address:            addr,
			SentPacketsCounter: mockSentPacketsCounter,
			LostPacketsCounter: mockLostPacketsCounter,
			PingHistogram:      mockPingHistogram,
			JitterHistogram:    mockJitterHistogram,
			Interval:           25 * time.Millisecond,
			Timeout:            25 * time.Millisecond,
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

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).AnyTimes()
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		go func() {
			err := client.Start(ctx)
			require.Error(t, err)
			require.Equal(t, context.Canceled.Error(), err.Error())
		}()

		cancel()
	})
}
