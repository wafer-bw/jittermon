package peer_test

//go:generate go run go.uber.org/mock/mockgen -source=peer_export_test.go -destination=peer_mocks_test.go -package=peer_test Recorder,PollServiceClient

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/peer"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("constructs a new peer without any options", func(t *testing.T) {
		t.Parallel()

		p, err := peer.NewPeer()
		require.NoError(t, err)
		require.NotNil(t, p)
	})

	t.Run("constructs a new peer when provided with nil options", func(t *testing.T) {
		t.Parallel()

		p, err := peer.NewPeer(peer.WithID("abc"), nil, nil)
		require.NoError(t, err)
		require.NotNil(t, p)
	})

	t.Run("executes option funcs", func(t *testing.T) {
		t.Parallel()

		executed := new(bool)
		_, err := peer.NewPeer(func(_ *peer.Peer) error {
			*executed = true
			return nil
		})
		require.NoError(t, err)
		require.True(t, *executed)
	})

	t.Run("returns error returned by option func", func(t *testing.T) {
		t.Parallel()

		_, err := peer.NewPeer(func(_ *peer.Peer) error { return fmt.Errorf("error") })
		require.Error(t, err)
	})

	t.Run("with id option sets & trims id", func(t *testing.T) {
		t.Parallel()

		id, expectedID := " id ", "id"

		p, err := peer.NewPeer()
		require.NoError(t, err)
		require.NotEqual(t, id, p.GetID())

		p, err = peer.NewPeer(peer.WithID(id))
		require.NoError(t, err)
		require.Equal(t, expectedID, p.GetID())
	})

	t.Run("with id option does not set id if empty", func(t *testing.T) {
		t.Parallel()

		id := ""
		p, err := peer.NewPeer(peer.WithID(id))
		require.NoError(t, err)
		require.NotEmpty(t, p.GetID())
	})

	t.Run("with logger option sets logger", func(t *testing.T) {
		t.Parallel()

		logger := new(slog.Logger)

		p, err := peer.NewPeer()
		require.NoError(t, err)
		require.NotEqual(t, logger, p.GetLogger())

		p, err = peer.NewPeer(peer.WithLogger(logger))
		require.NoError(t, err)
		require.Equal(t, logger, p.GetLogger())
	})

	t.Run("with recorders option sets recorders", func(t *testing.T) {
		t.Parallel()

		executed := new(bool)

		rnoop := func(_ recorder.Recorder) recorder.Recorder {
			return recorder.RecorderFunc(func(_ context.Context, _ recorder.Sample) {})
		}

		rset := func(next recorder.Recorder) recorder.Recorder {
			return recorder.RecorderFunc(func(ctx context.Context, s recorder.Sample) {
				*executed = true
			})
		}

		p, err := peer.NewPeer(peer.WithRecorders(rnoop))
		require.NoError(t, err)
		p.GetRecorder().Record(context.Background(), recorder.Sample{})
		require.False(t, *executed)

		p, err = peer.NewPeer(peer.WithRecorders(rset))
		require.NoError(t, err)
		p.GetRecorder().Record(context.Background(), recorder.Sample{})
		require.True(t, *executed)
	})
}

func TestPoll(t *testing.T) {
	t.Parallel()

	t.Run("handles incoming poll requests and responds with id", func(t *testing.T) {
		t.Parallel()

		id1, id2 := "id1", "id2"

		p, err := peer.NewPeer(peer.WithID(id1))
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		req.SetId(id2)
		req.SetTimestamp(timestamppb.Now())
		res, err := p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, id1, res.GetId())
		require.Nil(t, res.GetJitter())
	})

	t.Run("responds with & records jitter between periodic requests", func(t *testing.T) {
		t.Parallel()

		id := "id"
		mockRecorder := NewMockRecorder(gomock.NewController(t))
		r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

		p, err := peer.NewPeer(peer.WithRecorders(r))
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		req.SetId(id)
		req.SetTimestamp(timestamppb.Now())
		res, err := p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Nil(t, res.GetJitter())

		time.Sleep(25 * time.Millisecond)

		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeDownstreamJitter, s.Type)
			require.Equal(t, id, s.Src)
			require.Equal(t, p.GetID(), s.Dst)
			require.NotZero(t, s.Val)
		}).Times(1)

		req = &pollpb.PollRequest{}
		req.SetId(id)
		req.SetTimestamp(timestamppb.Now())
		res, err = p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.NotNil(t, res.GetJitter())
		require.Greater(t, res.GetJitter().AsDuration().Nanoseconds(), int64(0))
	})

	t.Run("records poll metrics", func(t *testing.T) {
		t.Parallel()

		mockRecorder := NewMockRecorder(gomock.NewController(t))

		r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

		p, err := peer.NewPeer(peer.WithRecorders(r))
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		req.SetId("id")
		req.SetTimestamp(timestamppb.Now())
		res, err := p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)

	})

	t.Run("returns error if no id is provided", func(t *testing.T) {
		t.Parallel()

		p, err := peer.NewPeer()
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		res, err := p.Poll(context.Background(), req)
		require.Error(t, err)
		require.Nil(t, res)
	})

	t.Run("returns error if no timestamp is provided", func(t *testing.T) {
		t.Parallel()

		p, err := peer.NewPeer()
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		req.SetId("id")
		res, err := p.Poll(context.Background(), req)
		require.Error(t, err)
		require.Nil(t, res)
	})
}

func TestDoPoll(t *testing.T) {
	t.Parallel()

	const (
		id   string = "id"
		addr string = "localhost"
	)

	t.Run("successfull poll", func(t *testing.T) {
		t.Parallel()

		mockClient := NewMockPollServiceClient(gomock.NewController(t))
		peer, err := peer.NewPeer()
		require.NoError(t, err)

		ctx := t.Context()
		resp := &pollpb.PollResponse{}
		resp.SetId(id)
		resp.SetJitter(durationpb.New(5 * time.Millisecond))

		mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

		err = peer.DoPoll(ctx, mockClient, addr)
		require.NoError(t, err)
	})

	t.Run("records successfull poll metrics", func(t *testing.T) {
		t.Parallel()

		mockRecorder := NewMockRecorder(gomock.NewController(t))
		mockClient := NewMockPollServiceClient(gomock.NewController(t))

		r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

		peer, err := peer.NewPeer(peer.WithRecorders(r))
		require.NoError(t, err)

		ctx := t.Context()
		resp := &pollpb.PollResponse{}
		resp.SetId(id)
		resp.SetJitter(durationpb.New(5 * time.Millisecond))

		mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeSentPackets, s.Type)
			require.Equal(t, peer.GetID(), s.Src)
			require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
			require.Equal(t, struct{}{}, s.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeUpstreamJitter, s.Type)
			require.Equal(t, peer.GetID(), s.Src)
			require.Equal(t, id, s.Dst)
			require.Equal(t, 5*time.Millisecond, s.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeRTT, s.Type)
			require.Equal(t, peer.GetID(), s.Src)
			require.Equal(t, id, s.Dst)
			require.NotZero(t, 5*time.Millisecond)
		}).Times(1)

		peer.DoPoll(ctx, mockClient, addr)
	})

	t.Run("records failed poll metrics", func(t *testing.T) {
		t.Parallel()

		mockRecorder := NewMockRecorder(gomock.NewController(t))
		mockClient := NewMockPollServiceClient(gomock.NewController(t))

		r := func(next recorder.Recorder) recorder.Recorder { return mockRecorder }

		peer, err := peer.NewPeer(peer.WithRecorders(r))
		require.NoError(t, err)

		ctx := t.Context()

		mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(&pollpb.PollResponse{}, fmt.Errorf("error"))
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeSentPackets, s.Type)
			require.Equal(t, peer.GetID(), s.Src)
			require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
			require.Equal(t, struct{}{}, s.Val)
		}).Times(1)
		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s recorder.Sample) {
			require.Equal(t, recorder.SampleTypeLostPackets, s.Type)
			require.Equal(t, peer.GetID(), s.Src)
			require.Equal(t, addr, s.Dst) // has to be addr bc we wont have id in this context.
			require.Equal(t, struct{}{}, s.Val)
		}).Times(1)

		peer.DoPoll(ctx, mockClient, addr)
	})

	t.Run("returns an error if no id is provided in response", func(t *testing.T) {
		t.Parallel()

		mockClient := NewMockPollServiceClient(gomock.NewController(t))
		peer, err := peer.NewPeer()
		require.NoError(t, err)

		ctx := t.Context()
		resp := &pollpb.PollResponse{}
		resp.SetJitter(durationpb.New(5 * time.Millisecond))

		mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

		err = peer.DoPoll(ctx, mockClient, addr)
		require.Error(t, err)
	})

	t.Run("returns an error if no jitter is provided in response", func(t *testing.T) {
		t.Parallel()

		mockClient := NewMockPollServiceClient(gomock.NewController(t))
		peer, err := peer.NewPeer()
		require.NoError(t, err)

		ctx := t.Context()
		resp := &pollpb.PollResponse{}
		resp.SetId(id)

		mockClient.EXPECT().Poll(ctx, gomock.Any(), gomock.Any()).Return(resp, nil)

		err = peer.DoPoll(ctx, mockClient, addr)
		require.Error(t, err)
	})
}
