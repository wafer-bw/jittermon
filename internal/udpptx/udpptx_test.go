package udpptx_test

//go:generate go run go.uber.org/mock/mockgen -source=udpptx.go -destination=udpptx_mocks_test.go -package=udpptx_test

import (
	"bytes"
	"context"
	"log/slog"
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

	t.Run("successful start", func(t *testing.T) {
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

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(2)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
		mockJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, _ float64, _ ...metric.RecordOption) {
			cancel()
		}).Times(1)

		err = client.Start(ctx)
		require.Error(t, err)
		require.Equal(t, context.Canceled.Error(), err.Error())
	})

	t.Run("logs poll errors", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

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
			Log:                logger,
			SentPacketsCounter: mockSentPacketsCounter,
			LostPacketsCounter: mockLostPacketsCounter,
			PingHistogram:      mockPingHistogram,
			JitterHistogram:    mockJitterHistogram,
		}

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
		gomock.InOrder(
			mockLostPacketsCounter.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()),
			mockLostPacketsCounter.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, _ int64, _ ...metric.AddOption) {
				cancel()
			}),
		)

		err = client.Start(ctx)
		require.Error(t, err)
		require.Equal(t, context.Canceled.Error(), err.Error())
		require.Contains(t, buf.String(), "poll failed")
	})

	t.Run("returns error on empty id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		port, err := freeport.GetFreePort()
		require.NoError(t, err)

		client := udpptx.Client{
			ID:                 "",
			Address:            net.JoinHostPort("", strconv.Itoa(port)),
			SentPacketsCounter: NewMockInt64Counter(gomock.NewController(t)),
			LostPacketsCounter: NewMockInt64Counter(gomock.NewController(t)),
			PingHistogram:      NewMockFloat64Histogram(gomock.NewController(t)),
			JitterHistogram:    NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err = client.Start(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on empty address", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		client := udpptx.Client{
			ID:                 "test",
			Address:            "",
			SentPacketsCounter: NewMockInt64Counter(gomock.NewController(t)),
			LostPacketsCounter: NewMockInt64Counter(gomock.NewController(t)),
			PingHistogram:      NewMockFloat64Histogram(gomock.NewController(t)),
			JitterHistogram:    NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err := client.Start(ctx)
		require.Error(t, err)
	})
}
