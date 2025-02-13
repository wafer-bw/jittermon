package peer_test

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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("constructs a new peer without any options", func(t *testing.T) {
		t.Parallel()

		p, err := peer.NewPeer(nil)
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

		p, err := peer.NewPeer(nil)
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
		req.SetTimestamp(timestamppb.New(time.Now()))
		res, err := p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, id1, res.GetId())
		require.Nil(t, res.GetJitter())
	})

	t.Run("responds with jitter between periodic requests", func(t *testing.T) {
		t.Parallel()

		id := "id"

		p, err := peer.NewPeer()
		require.NoError(t, err)

		req := &pollpb.PollRequest{}
		req.SetId(id)
		req.SetTimestamp(timestamppb.New(time.Now()))
		res, err := p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Nil(t, res.GetJitter())

		time.Sleep(25 * time.Millisecond)

		req = &pollpb.PollRequest{}
		req.SetId(id)
		req.SetTimestamp(timestamppb.New(time.Now()))
		res, err = p.Poll(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.NotNil(t, res.GetJitter())
		require.Greater(t, res.GetJitter().AsDuration().Nanoseconds(), int64(0))
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

	// TODO: test cases
}
