package grpcptp_test

// //go:generate go run go.uber.org/mock/mockgen -source=grpc.go -destination=grpc_mocks_test.go -package=grpcp2platency_test

// import (
// 	"context"
// 	"errors"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/require"
// 	pollpb "github.com/wafer-bw/jittermon/internal/gen/go/poll/v1"
// 	"github.com/wafer-bw/jittermon/internal/recorder"
// 	"github.com/wafer-bw/jittermon/internal/sampler/grpcp2platency"
// 	"go.uber.org/mock/gomock"
// 	"google.golang.org/protobuf/types/known/durationpb"
// 	"google.golang.org/protobuf/types/known/timestamppb"
// )

// func TestClient_Poll(t *testing.T) {
// 	t.Parallel()

// 	addr := "localhost:12345"

// 	t.Run("successful poll", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		start := time.Now()
// 		jitter := 5 * time.Millisecond
// 		mockClient := NewMockClientPoller(gomock.NewController(t))
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		client, err := grpcp2platency.NewClient(addr, mockRecorder, grpcp2platency.WithClientConn(mockClient))
// 		require.NoError(t, err)

// 		resp := &pollpb.PollResponse{}
// 		resp.SetId("server")
// 		resp.SetJitter(durationpb.New(jitter))

// 		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeSentPackets, sample.Type)
// 			require.Equal(t, struct{}{}, sample.Val)
// 		}).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeRTT, sample.Type)
// 			require.NotZero(t, sample.Val)
// 			v, ok := sample.Val.(time.Duration)
// 			require.True(t, ok)
// 			require.Less(t, v, time.Since(start))
// 		}).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeUpstreamJitter, sample.Type)
// 			require.Equal(t, jitter, sample.Val)
// 		}).Times(1)

// 		err = client.Poll(ctx)
// 		require.NoError(t, err)
// 	})

// 	t.Run("failed poll", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		jitter := 5 * time.Millisecond
// 		mockClient := NewMockClientPoller(gomock.NewController(t))
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		client, err := grpcp2platency.NewClient(addr, mockRecorder, grpcp2platency.WithClientConn(mockClient))
// 		require.NoError(t, err)

// 		resp := &pollpb.PollResponse{}
// 		resp.SetId("server")
// 		resp.SetJitter(durationpb.New(jitter))

// 		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeSentPackets, sample.Type)
// 			require.Equal(t, struct{}{}, sample.Val)
// 		}).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeLostPackets, sample.Type)
// 			require.Equal(t, struct{}{}, sample.Val)
// 		}).Times(1)

// 		err = client.Poll(ctx)
// 		require.Error(t, err)
// 	})

// 	t.Run("returns error on missing response id", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		jitter := 5 * time.Millisecond
// 		mockClient := NewMockClientPoller(gomock.NewController(t))
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		client, err := grpcp2platency.NewClient(addr, mockRecorder, grpcp2platency.WithClientConn(mockClient))
// 		require.NoError(t, err)

// 		resp := &pollpb.PollResponse{}
// 		resp.SetJitter(durationpb.New(jitter))

// 		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Times(2)

// 		err = client.Poll(ctx)
// 		require.Error(t, err)
// 	})

// 	t.Run("returns error on missing response jitter", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		mockClient := NewMockClientPoller(gomock.NewController(t))
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		client, err := grpcp2platency.NewClient(addr, mockRecorder, grpcp2platency.WithClientConn(mockClient))
// 		require.NoError(t, err)

// 		resp := &pollpb.PollResponse{}
// 		resp.SetId("server")

// 		mockClient.EXPECT().Poll(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Times(2)

// 		err = client.Poll(ctx)
// 		require.Error(t, err)
// 	})
// }

// func TestServer_Poll(t *testing.T) {
// 	t.Parallel()

// 	addr := "localhost:12345"

// 	t.Run("successful poll", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		start := time.Now()
// 		clientID, serverID := "client", "server"
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		server, err := grpcp2platency.NewServer(addr, mockRecorder, grpcp2platency.WithServerID(serverID))
// 		require.NoError(t, err)

// 		req := &pollpb.PollRequest{}
// 		req.SetId(clientID)
// 		req.SetTimestamp(timestamppb.New(start))

// 		jitter := time.Duration(0)
// 		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, sample recorder.Sample) {
// 			require.Equal(t, recorder.SampleTypeDownstreamJitter, sample.Type)
// 			v, ok := sample.Val.(time.Duration)
// 			require.True(t, ok)
// 			require.NotZero(t, v)
// 			jitter = v
// 		}).Times(1)

// 		res, err := server.Poll(ctx, req)
// 		require.NoError(t, err)
// 		require.Equal(t, serverID, res.GetId())
// 		require.Nil(t, res.GetJitter()) // no jitter on the first poll.

// 		time.Sleep(5 * time.Millisecond)

// 		res, err = server.Poll(ctx, req)
// 		require.NoError(t, err)
// 		require.Equal(t, serverID, res.GetId())
// 		require.NotNil(t, res.GetJitter())
// 		require.Equal(t, jitter, res.GetJitter().AsDuration())
// 	})

// 	t.Run("return error when id is missing", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		start := time.Now()
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		server, err := grpcp2platency.NewServer(addr, mockRecorder)
// 		require.NoError(t, err)

// 		req := &pollpb.PollRequest{}
// 		req.SetTimestamp(timestamppb.New(start))

// 		_, err = server.Poll(ctx, req)
// 		require.Error(t, err)
// 	})

// 	t.Run("return error when timestamp is missing", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		mockRecorder := NewMockRecorder(gomock.NewController(t))

// 		server, err := grpcp2platency.NewServer(addr, mockRecorder)
// 		require.NoError(t, err)

// 		req := &pollpb.PollRequest{}
// 		req.SetId("client")

// 		_, err = server.Poll(ctx, req)
// 		require.Error(t, err)
// 	})
// }
