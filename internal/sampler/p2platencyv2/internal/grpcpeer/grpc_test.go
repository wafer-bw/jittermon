package grpcpeer_test

//go:generate go run go.uber.org/mock/mockgen -source=grpc.go -destination=grpc_mocks_test.go -package=grpcpeer_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platencyv2/internal/grpcpeer"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platencyv2/internal/grpcpeer/pollpb"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

		client := &grpcpeer.Client{
			ID:       "client",
			Address:  addr,
			Interval: 1 * time.Second,
			Log:      slog.New(slog.DiscardHandler),
			Client:   mockClient,
			SampleCh: make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		resp := &pollpb.PollResponse{}
		resp.SetId("server")
		resp.SetJitter(durationpb.New(jitter))

		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)

		err := client.Poll(ctx)
		require.NoError(t, err)

		samples := client.Samples()

		sample := <-samples
		require.Equal(t, rec.SampleTypeSentPackets, sample.Type)
		require.Equal(t, struct{}{}, sample.Val)

		sample = <-samples
		require.Equal(t, rec.SampleTypeUpstreamJitter, sample.Type)
		require.Equal(t, jitter, sample.Val)

		sample = <-samples
		require.Equal(t, rec.SampleTypeRTT, sample.Type)
		require.NotZero(t, sample.Val)
		v, ok := sample.Val.(time.Duration)
		require.True(t, ok)
		require.Less(t, v, time.Since(start))
	})

	t.Run("unsuccessful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		jitter := 5 * time.Millisecond
		mockClient := NewMockClientPoller(gomock.NewController(t))

		client := &grpcpeer.Client{
			ID:       "client",
			Address:  addr,
			Interval: 1 * time.Second,
			Log:      slog.New(slog.DiscardHandler),
			Client:   mockClient,
			SampleCh: make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		resp := &pollpb.PollResponse{}
		resp.SetId("server")
		resp.SetJitter(durationpb.New(jitter))

		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)

		samples := client.Samples()

		sample := <-samples
		require.Equal(t, rec.SampleTypeSentPackets, sample.Type)
		require.Equal(t, struct{}{}, sample.Val)

		sample = <-samples
		require.Equal(t, rec.SampleTypeLostPackets, sample.Type)
		require.Equal(t, struct{}{}, sample.Val)
	})

	t.Run("returns error on missing response id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		jitter := 5 * time.Millisecond
		mockClient := NewMockClientPoller(gomock.NewController(t))

		client := &grpcpeer.Client{
			ID:       "client",
			Address:  addr,
			Interval: 1 * time.Second,
			Log:      slog.New(slog.DiscardHandler),
			Client:   mockClient,
			SampleCh: make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		resp := &pollpb.PollResponse{}
		resp.SetJitter(durationpb.New(jitter))

		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)
	})

	t.Run("returns error on missing response jitter", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockClient := NewMockClientPoller(gomock.NewController(t))

		client := &grpcpeer.Client{
			ID:       "client",
			Address:  addr,
			Interval: 1 * time.Second,
			Log:      slog.New(slog.DiscardHandler),
			Client:   mockClient,
			SampleCh: make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		resp := &pollpb.PollResponse{}
		resp.SetId("server")

		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)

		err := client.Poll(ctx)
		require.Error(t, err)
	})
}

func TestClient_Start(t *testing.T) {
	t.Parallel()

	addr := "localhost:12345"

	t.Run("starts poll tick loop successfully", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		mockClient := NewMockClientPoller(gomock.NewController(t))

		client := &grpcpeer.Client{
			ID:            "client",
			Address:       addr,
			Interval:      10 * time.Millisecond,
			Client:        mockClient,
			ClientOptions: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			Log:           slog.New(slog.DiscardHandler),
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
			SampleCh:      make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ *pollpb.PollRequest, _ ...grpc.CallOption) (*pollpb.PollResponse, error) {
			close(client.StopCh)
			return nil, errors.New("error")
		}).Times(1)

		err := client.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("exits when stop channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		client := &grpcpeer.Client{
			ID:            "client",
			Address:       addr,
			Interval:      1 * time.Second,
			ClientOptions: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			Log:           slog.New(slog.DiscardHandler),
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
			SampleCh:      make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		close(client.StopCh)

		err := client.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := &grpcpeer.Client{
			ID:            "client",
			Address:       addr,
			Interval:      1 * time.Second,
			ClientOptions: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			Log:           slog.New(slog.DiscardHandler),
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
			SampleCh:      make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		err := client.Start(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fails when client creation fails", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		client := &grpcpeer.Client{
			ID:            "client",
			Address:       addr,
			Interval:      1 * time.Second,
			ClientOptions: []grpc.DialOption{}, // invokes desired failure.
			Log:           slog.New(slog.DiscardHandler),
			StopCh:        make(chan struct{}),
			StoppedCh:     make(chan struct{}),
			SampleCh:      make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		err := client.Start(ctx)
		require.Error(t, err)
	})
}

func TestClient_Stop(t *testing.T) {
	t.Parallel()

	t.Run("successful stop when stopped channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		client := &grpcpeer.Client{
			Log:       slog.New(slog.DiscardHandler),
			StopCh:    make(chan struct{}),
			StoppedCh: make(chan struct{}),
			SampleCh:  make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		close(client.StoppedCh)

		err := client.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("fails when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := &grpcpeer.Client{
			Log:       slog.New(slog.DiscardHandler),
			StopCh:    make(chan struct{}),
			StoppedCh: make(chan struct{}),
			SampleCh:  make(chan recorder.Sample, grpcpeer.RecommendedClientChannelBufferSize),
		}

		err := client.Stop(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestServer_Poll(t *testing.T) {
	t.Parallel()

	addr := "localhost:12345"

	t.Run("successful poll", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		start := time.Now()
		clientID, serverID := "client", "server"

		server := &grpcpeer.Server{
			ID:             serverID,
			Address:        addr,
			Proto:          "tcp",
			RequestBuffers: &jitter.Buffer{},
			Log:            slog.New(slog.DiscardHandler),
			StartedCh:      make(chan struct{}),
			StoppedCh:      make(chan struct{}),
			SampleCh:       make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		req := &pollpb.PollRequest{}
		req.SetId(clientID)
		req.SetTimestamp(timestamppb.New(start))

		res, err := server.Poll(ctx, req)
		require.NoError(t, err)
		require.Equal(t, serverID, res.GetId())
		require.Nil(t, res.GetJitter()) // no jitter on the first poll.

		time.Sleep(5 * time.Millisecond)

		res, err = server.Poll(ctx, req)
		require.NoError(t, err)
		require.Equal(t, serverID, res.GetId())
		require.NotNil(t, res.GetJitter())
		j := res.GetJitter().AsDuration()
		require.NotZero(t, j)

		sample := <-server.Samples()

		require.Equal(t, rec.SampleTypeDownstreamJitter, sample.Type)
		v, ok := sample.Val.(time.Duration)
		require.True(t, ok)
		require.Equal(t, j, v)
	})

	t.Run("return error when id is missing", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		start := time.Now()

		server := &grpcpeer.Server{
			ID:             "server",
			Address:        addr,
			Proto:          "tcp",
			RequestBuffers: &jitter.Buffer{},
			Log:            slog.New(slog.DiscardHandler),
			StartedCh:      make(chan struct{}),
			StoppedCh:      make(chan struct{}),
			SampleCh:       make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		req := &pollpb.PollRequest{}
		req.SetTimestamp(timestamppb.New(start))

		_, err := server.Poll(ctx, req)
		require.Error(t, err)
		// TODO: check ErrorAs expected.
	})

	t.Run("return error when timestamp is missing", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := &grpcpeer.Server{
			ID:             "server",
			Address:        addr,
			Proto:          "tcp",
			RequestBuffers: &jitter.Buffer{},
			Log:            slog.New(slog.DiscardHandler),
			StartedCh:      make(chan struct{}),
			StoppedCh:      make(chan struct{}),
			SampleCh:       make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		req := &pollpb.PollRequest{}
		req.SetId("client")

		_, err := server.Poll(ctx, req)
		require.Error(t, err)
		// TODO: check ErrorAs expected.
	})
}

func TestServer_Start(t *testing.T) {
	t.Parallel()

	t.Run("successful start", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := &grpcpeer.Server{
			Proto:                   "tcp",
			Address:                 "localhost:12346",
			ServerReflectionEnabled: true,
			Log:                     slog.New(slog.DiscardHandler),
			StartedCh:               make(chan struct{}),
			StoppedCh:               make(chan struct{}),
			SampleCh:                make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		assertCh := make(chan struct{})
		go func() {
			err := server.Start(ctx)
			require.NoError(t, err)
			close(assertCh)
		}()

		<-server.StartedCh
		server.Server.Stop()
		<-assertCh
	})

	t.Run("fails to start when listener fails to start", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		_, err := net.Listen("", "")
		require.Error(t, err)

		server := &grpcpeer.Server{
			Proto:                   "",
			Address:                 "",
			ServerReflectionEnabled: true,
			Log:                     slog.New(slog.DiscardHandler),
			StartedCh:               make(chan struct{}),
			StoppedCh:               make(chan struct{}),
			SampleCh:                make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		assertCh := make(chan struct{})
		go func() {
			err := server.Start(ctx)
			require.Error(t, err)
			close(assertCh)
		}()

		<-assertCh
	})
}

func TestServer_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stops when stopped channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := &grpcpeer.Server{
			Log:       slog.New(slog.DiscardHandler),
			Server:    grpc.NewServer(),
			StartedCh: make(chan struct{}),
			StoppedCh: make(chan struct{}),
			SampleCh:  make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		close(server.StoppedCh)

		err := server.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("stops when started channel is closed", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		server := &grpcpeer.Server{
			Log:       slog.New(slog.DiscardHandler),
			Server:    grpc.NewServer(),
			StartedCh: make(chan struct{}),
			StoppedCh: make(chan struct{}),
			SampleCh:  make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		close(server.StartedCh)

		err := server.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("stop fails when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		server := &grpcpeer.Server{
			Log:       slog.New(slog.DiscardHandler),
			Server:    grpc.NewServer(),
			StartedCh: make(chan struct{}),
			StoppedCh: make(chan struct{}),
			SampleCh:  make(chan recorder.Sample, grpcpeer.RecommendedServerChannelBufferSize),
		}

		err := server.Stop(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}
