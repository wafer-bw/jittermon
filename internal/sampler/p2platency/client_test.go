package p2platency_test

//go:generate go run go.uber.org/mock/mockgen -source=client.go -destination=client_mocks_test.go -package=p2platency_test Recorder,Poller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency"
	gomock "go.uber.org/mock/gomock"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("constructs a new peer without any options", func(t *testing.T) {
		t.Parallel()

		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(1)

		client, err := p2platency.NewClient(mockPoller)
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("constructs without panic when provided nil options", func(t *testing.T) {
		t.Parallel()

		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(1)

		require.NotPanics(t, func() {
			client, err := p2platency.NewClient(mockPoller, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	})

	t.Run("executes option funcs", func(t *testing.T) {
		t.Parallel()

		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(1)
		executed := new(bool)

		_, err := p2platency.NewClient(mockPoller, func(_ *p2platency.Client) error {
			*executed = true
			return nil
		})
		require.NoError(t, err)
		require.True(t, *executed)
	})

	t.Run("returns error returned by option func", func(t *testing.T) {
		t.Parallel()

		mockPoller := NewMockPoller(gomock.NewController(t))

		_, err := p2platency.NewClient(mockPoller, func(_ *p2platency.Client) error {
			return fmt.Errorf("error")
		})
		require.Error(t, err)
	})

	t.Run("id option sets id", func(t *testing.T) {
		t.Parallel()

		id, expectedID := " id ", "id"
		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(3)

		client, err := p2platency.NewClient(mockPoller)
		require.NoError(t, err)
		require.NotEqual(t, id, client.GetID())

		client, err = p2platency.NewClient(mockPoller, p2platency.ClientID(id))
		require.NoError(t, err)
		require.Equal(t, expectedID, client.GetID())

		id = ""
		client, err = p2platency.NewClient(mockPoller, p2platency.ClientID(id))
		require.NoError(t, err)
		require.NotEmpty(t, client.GetID())
	})

	t.Run("log option sets logger", func(t *testing.T) {
		t.Parallel()

		msg := "test123"
		buf := bytes.NewBuffer([]byte{})
		log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{}))
		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(2)

		client, err := p2platency.NewClient(mockPoller)
		require.NoError(t, err)
		l1 := client.GetLogger()
		l1.Info(msg)
		require.Empty(t, buf.String())

		client, err = p2platency.NewClient(mockPoller, p2platency.ClientLog(log))
		require.NoError(t, err)
		l2 := client.GetLogger()
		l2.Info(msg)
		require.Contains(t, buf.String(), msg)
	})

	t.Run("recorder option sets recorder", func(t *testing.T) {
		t.Parallel()

		executed := new(bool)
		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(2)
		rset := recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) { *executed = true })

		client, err := p2platency.NewClient(mockPoller)
		require.NoError(t, err)
		client.GetRecorder().Record(t.Context(), recorder.Sample{})
		require.False(t, *executed)

		client, err = p2platency.NewClient(mockPoller, p2platency.ClientRecorder(rset))
		require.NoError(t, err)
		client.GetRecorder().Record(t.Context(), recorder.Sample{})
		require.True(t, *executed)
	})

	t.Run("interval option sets interval", func(t *testing.T) {
		t.Parallel()

		interval := 1 * time.Second
		mockPoller := NewMockPoller(gomock.NewController(t))
		mockPoller.EXPECT().Address().Return("localhost:8080").Times(2)

		client, err := p2platency.NewClient(mockPoller, p2platency.ClientInterval(interval))
		require.NoError(t, err)
		require.Equal(t, interval, client.GetInterval())

		interval = time.Duration(0)
		client, err = p2platency.NewClient(mockPoller, p2platency.ClientInterval(interval))
		require.NoError(t, err)
		require.Equal(t, p2platency.DefaultInterval, client.GetInterval())
	})
}

func TestClient_Sample(t *testing.T) {
	t.Parallel()

	addr := "localhost:8080"
	_ = addr

	// t.Run("successful sample", func(t *testing.T) {
	// 	t.Parallel()

	// 	clientID, serverID := "client", "server"
	// 	mockClient := NewMockPollServiceClient(gomock.NewController(t))
	// 	clientFunc := func(string, ...grpc.DialOption) (pollpb.PollServiceClient, func() error, error) {
	// 		return mockClient, func() error { return nil }, nil
	// 	}

	// 	client, err := p2platency.NewClient(mockPoller,
	// 		p2platency.ClientID(clientID),
	// 		p2platency.WithClientFunc(clientFunc),
	// 	)
	// 	require.NoError(t, err)

	// 	ctx := t.Context()
	// 	resp := &pollpb.PollResponse{}
	// 	resp.SetId(serverID)
	// 	resp.SetJitter(durationpb.New(5 * time.Millisecond))

	// 	mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

	// 	err = client.Sample(ctx, addr)
	// 	require.NoError(t, err)
	// })

	// t.Run("records successful poll metrics", func(t *testing.T) {
	// 	t.Parallel()

	// 	mockRecorder := NewMockRecorder(gomock.NewController(t))
	// 	mockClient := NewMockPollServiceClient(gomock.NewController(t))

	// 	r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

	// 	peer, err := peer.NewPeer(peer.WithRecorders(r))
	// 	require.NoError(t, err)

	// 	ctx := t.Context()
	// 	resp := &pollpb.PollResponse{}
	// 	resp.SetId(id)
	// 	resp.SetJitter(durationpb.New(5 * time.Millisecond))

	// 	mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)
	// 	mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
	// 		require.Equal(t, recorder.SampleTypeSentPackets, s.Type)
	// 		require.Equal(t, peer.GetID(), s.Src)
	// 		require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
	// 		require.Equal(t, struct{}{}, s.Val)
	// 	}).Times(1)
	// 	mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
	// 		require.Equal(t, recorder.SampleTypeUpstreamJitter, s.Type)
	// 		require.Equal(t, peer.GetID(), s.Src)
	// 		require.Equal(t, id, s.Dst)
	// 		require.Equal(t, 5*time.Millisecond, s.Val)
	// 	}).Times(1)
	// 	mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
	// 		require.Equal(t, recorder.SampleTypeRTT, s.Type)
	// 		require.Equal(t, peer.GetID(), s.Src)
	// 		require.Equal(t, id, s.Dst)
	// 		require.NotZero(t, 5*time.Millisecond)
	// 	}).Times(1)

	// 	err = peer.DoPoll(ctx, mockClient, addr)
	// 	require.NoError(t, err)
	// })

	// t.Run("records failed poll metrics", func(t *testing.T) {
	// 	t.Parallel()

	// 	mockRecorder := NewMockRecorder(gomock.NewController(t))
	// 	mockClient := NewMockPollServiceClient(gomock.NewController(t))

	// 	r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

	// 	peer, err := peer.NewPeer(peer.WithRecorders(r))
	// 	require.NoError(t, err)

	// 	ctx := t.Context()

	// 	mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(&pollpb.PollResponse{}, fmt.Errorf("error"))
	// 	mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
	// 		require.Equal(t, recorder.SampleTypeSentPackets, s.Type)
	// 		require.Equal(t, peer.GetID(), s.Src)
	// 		require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
	// 		require.Equal(t, struct{}{}, s.Val)
	// 	}).Times(1)
	// 	mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
	// 		require.Equal(t, recorder.SampleTypeLostPackets, s.Type)
	// 		require.Equal(t, peer.GetID(), s.Src)
	// 		require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
	// 		require.Equal(t, struct{}{}, s.Val)
	// 	}).Times(1)

	// 	err = peer.DoPoll(ctx, mockClient, addr)
	// 	require.Error(t, err)
	// })

	// t.Run("returns an error if no id is provided in response", func(t *testing.T) {
	// 	t.Parallel()

	// 	mockClient := NewMockPollServiceClient(gomock.NewController(t))
	// 	client, err := p2platency.NewClient(mockPoller)
	// 	require.NoError(t, err)

	// 	ctx := t.Context()
	// 	resp := &pollpb.PollResponse{}
	// 	resp.SetJitter(durationpb.New(5 * time.Millisecond))

	// 	mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

	// 	err = peer.DoPoll(ctx, mockClient, addr)
	// 	require.Error(t, err)
	// })

	// t.Run("returns an error if no jitter is provided in response", func(t *testing.T) {
	// 	t.Parallel()

	// 	mockClient := NewMockPollServiceClient(gomock.NewController(t))
	// 	client, err := p2platency.NewClient(mockPoller)
	// 	require.NoError(t, err)

	// 	ctx := t.Context()
	// 	resp := &pollpb.PollResponse{}
	// 	resp.SetId(id)

	// 	mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

	// 	err = peer.DoPoll(ctx, mockClient, addr)
	// 	require.Error(t, err)
	// })
}
