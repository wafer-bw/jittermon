package grpcptp_test

//go:generate go run go.uber.org/mock/mockgen -source=grpcptp.go -destination=grpcptp_mocks_test.go -package=grpcptp_test

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	pollpb "github.com/wafer-bw/jittermon/internal/gen/go/poll/v1"
	"github.com/wafer-bw/jittermon/internal/grpcptp"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClient_Poll(t *testing.T) {
	t.Parallel()

	addr := "localhost:12345"

	t.Run("successful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		start := time.Now()
		jitter := 5 * time.Millisecond
		mockClient := NewMockClientPoller(gomock.NewController(t))
		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockUpstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		client := grpcptp.Client{
			ID:                      "test",
			Address:                 addr,
			SentPacketsCounter:      mockSentPacketsCounter,
			LostPacketsCounter:      mockLostPacketsCounter,
			PingHistogram:           mockPingHistogram,
			UpstreamJitterHistogram: mockUpstreamJitterHistogram,
			Conn:                    mockClient,
			Timeout:                 25 * time.Millisecond,
		}
		resp := pollpb.PollResponse_builder{
			Id:     new("server"),
			Jitter: durationpb.New(jitter),
		}.Build()

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, val float64, opts ...metric.RecordOption) {
			require.Less(t, val, time.Since(start).Seconds())
			require.NotZero(t, val)
		}).Times(1)
		mockUpstreamJitterHistogram.EXPECT().Record(gomock.Any(), jitter.Seconds(), gomock.Any()).Times(1)

		err := client.Poll(ctx)
		require.NoError(t, err)
	})

	t.Run("failed poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		mockClient := NewMockClientPoller(gomock.NewController(t))
		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockUpstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		client := grpcptp.Client{
			ID:                      "test",
			Address:                 addr,
			SentPacketsCounter:      mockSentPacketsCounter,
			LostPacketsCounter:      mockLostPacketsCounter,
			PingHistogram:           mockPingHistogram,
			UpstreamJitterHistogram: mockUpstreamJitterHistogram,
			Conn:                    mockClient,
			Timeout:                 25 * time.Millisecond,
		}

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
		mockLostPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on missing response id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		jitter := 5 * time.Millisecond
		mockClient := NewMockClientPoller(gomock.NewController(t))
		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockUpstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		client := grpcptp.Client{
			ID:                      "test",
			Address:                 addr,
			SentPacketsCounter:      mockSentPacketsCounter,
			LostPacketsCounter:      mockLostPacketsCounter,
			PingHistogram:           mockPingHistogram,
			UpstreamJitterHistogram: mockUpstreamJitterHistogram,
			Conn:                    mockClient,
			Timeout:                 25 * time.Millisecond,
		}
		resp := pollpb.PollResponse_builder{Jitter: durationpb.New(jitter)}.Build()

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on missing response jitter", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		mockClient := NewMockClientPoller(gomock.NewController(t))
		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockUpstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		client := grpcptp.Client{
			ID:                      "test",
			Address:                 addr,
			SentPacketsCounter:      mockSentPacketsCounter,
			LostPacketsCounter:      mockLostPacketsCounter,
			PingHistogram:           mockPingHistogram,
			UpstreamJitterHistogram: mockUpstreamJitterHistogram,
			Conn:                    mockClient,
			Timeout:                 25 * time.Millisecond,
		}
		resp := pollpb.PollResponse_builder{Id: new("server")}.Build()

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), int64(1), gomock.Any()).Times(1)
		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)
	})
}

func TestClient_Start(t *testing.T) {
	t.Parallel()

	t.Run("successful start", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mockClient := NewMockClientPoller(gomock.NewController(t))
		mockSentPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockLostPacketsCounter := NewMockInt64Counter(gomock.NewController(t))
		mockPingHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		mockUpstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		client := grpcptp.Client{
			ID:                      "test",
			Address:                 "localhost:12345",
			SentPacketsCounter:      mockSentPacketsCounter,
			LostPacketsCounter:      mockLostPacketsCounter,
			PingHistogram:           mockPingHistogram,
			UpstreamJitterHistogram: mockUpstreamJitterHistogram,
			Conn:                    mockClient,
		}

		mockSentPacketsCounter.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		mockPingHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockUpstreamJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		go func() {
			err := client.Start(ctx)
			require.Error(t, err)
			require.Equal(t, context.Canceled.Error(), err.Error())
		}()

		cancel()
	})

	t.Run("returns error on empty id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		port, err := freeport.GetFreePort()
		require.NoError(t, err)

		client := grpcptp.Client{
			ID:                      "",
			Address:                 net.JoinHostPort("", strconv.Itoa(port)),
			SentPacketsCounter:      NewMockInt64Counter(gomock.NewController(t)),
			LostPacketsCounter:      NewMockInt64Counter(gomock.NewController(t)),
			PingHistogram:           NewMockFloat64Histogram(gomock.NewController(t)),
			UpstreamJitterHistogram: NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err = client.Start(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on empty address", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		client := grpcptp.Client{
			ID:                      "test",
			Address:                 "",
			SentPacketsCounter:      NewMockInt64Counter(gomock.NewController(t)),
			LostPacketsCounter:      NewMockInt64Counter(gomock.NewController(t)),
			PingHistogram:           NewMockFloat64Histogram(gomock.NewController(t)),
			UpstreamJitterHistogram: NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err := client.Start(ctx)
		require.Error(t, err)
	})
}

func TestServer_Poll(t *testing.T) {
	t.Parallel()

	addr := "localhost:12345"

	t.Run("successful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		jitter := float64(0)
		start := time.Now()
		clientID, serverID := "client", "server"
		mockDownstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		server := grpcptp.Server{
			ID:                        serverID,
			Address:                   addr,
			DownstreamJitterHistogram: mockDownstreamJitterHistogram,
		}
		req := pollpb.PollRequest_builder{
			Id:        &clientID,
			Timestamp: timestamppb.New(start),
		}.Build()

		mockDownstreamJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, val float64, opts ...metric.RecordOption) {
			require.Less(t, val, time.Since(start).Seconds())
			require.NotZero(t, val)
			jitter = val
		}).Times(1)

		res, err := server.Poll(ctx, req)
		require.NoError(t, err)
		require.Equal(t, serverID, res.GetId())
		require.Nil(t, res.GetJitter()) // no jitter on the first poll.

		time.Sleep(5 * time.Millisecond)

		res, err = server.Poll(ctx, req)
		require.NoError(t, err)
		require.Equal(t, serverID, res.GetId())
		require.NotNil(t, res.GetJitter())
		require.Equal(t, jitter, res.GetJitter().AsDuration().Seconds())
	})

	t.Run("return error when id is missing", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		start := time.Now()
		mockDownstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		server := grpcptp.Server{
			ID:                        "server",
			Address:                   addr,
			DownstreamJitterHistogram: mockDownstreamJitterHistogram,
		}

		req := pollpb.PollRequest_builder{Timestamp: timestamppb.New(start)}.Build()

		_, err := server.Poll(ctx, req)
		require.Error(t, err)
	})

	t.Run("return error when timestamp is missing", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		mockDownstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		server := grpcptp.Server{
			ID:                        "server",
			Address:                   addr,
			DownstreamJitterHistogram: mockDownstreamJitterHistogram,
		}

		req := pollpb.PollRequest_builder{Id: new("client")}.Build()

		_, err := server.Poll(ctx, req)
		require.Error(t, err)
	})
}

func TestServer_Start(t *testing.T) {
	t.Parallel()

	t.Run("successful start", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mockDownstreamJitterHistogram := NewMockFloat64Histogram(gomock.NewController(t))
		server := grpcptp.Server{
			ID:                        "server",
			Address:                   "localhost:12345",
			DownstreamJitterHistogram: mockDownstreamJitterHistogram,
		}

		mockDownstreamJitterHistogram.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		go func() {
			err := server.Start(ctx)
			require.Error(t, err)
			require.Equal(t, context.Canceled.Error(), err.Error())
		}()

		cancel()
	})

	t.Run("returns error on empty id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := grpcptp.Server{
			ID:                        "",
			Address:                   "localhost:12345",
			DownstreamJitterHistogram: NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err := server.Start(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on empty address", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := grpcptp.Server{
			ID:                        "server",
			Address:                   "",
			DownstreamJitterHistogram: NewMockFloat64Histogram(gomock.NewController(t)),
		}

		err := server.Start(ctx)
		require.Error(t, err)
	})
}
