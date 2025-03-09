package p2platency_test

//go:generate go run go.uber.org/mock/mockgen -source=peer.go -destination=peer_mocks_test.go -package=p2platency_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
)

func TestNewPeer(t *testing.T) {
	t.Parallel()

	t.Run("successfully creates new populated peer", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		log := new(slog.Logger)

		recorderCalled := false
		rec := recorder.RecorderFunc(func(_ context.Context, _ recorder.Sample) {
			recorderCalled = true
		})

		p, err := p2platency.NewPeer(
			p2platency.WithID(" id "),
			p2platency.WithInterval(250*time.Millisecond),
			p2platency.WithListenAddress("localhost:8080"),
			p2platency.WithSendAddresses("localhost:8081", "localhost:8082"),
			p2platency.WithGRPC("tcp", []grpc.DialOption{}, []grpc.ServerOption{}, true),
			p2platency.WithRecorder(rec),
			p2platency.WithLog(log),
		)
		require.NoError(t, err)
		require.NotNil(t, p)

		grpcConf := p.GetGRPCConfig()
		p.GetRecorder().Record(ctx, recorder.Sample{})
		require.True(t, recorderCalled)

		require.Len(t, p.GetGroup(), 3)
		require.Equal(t, "id", p.GetID())
		require.Equal(t, 250*time.Millisecond, p.GetInterval())
		require.Equal(t, "localhost:8080", p.GetListenAddress())
		require.Equal(t, []string{"localhost:8081", "localhost:8082"}, p.GetSendAddresses())
		require.Equal(t, p2platency.ModeGRPC, p.GetMode())
		require.Equal(t, "tcp", grpcConf.Proto)
		require.NotNil(t, grpcConf.ClientOptions)
		require.Empty(t, grpcConf.ClientOptions)
		require.NotNil(t, grpcConf.ServerOptions)
		require.Empty(t, grpcConf.ServerOptions)
		require.True(t, grpcConf.Reflection)
		require.Equal(t, log, p.GetLog())
	})

	t.Run("successfully creates new empty peer", func(t *testing.T) {
		t.Parallel()

		p, err := p2platency.NewPeer()
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Empty(t, p.GetGroup())
	})

	t.Run("successfully creates new empty peer when given blank id option", func(t *testing.T) {
		t.Parallel()

		p, err := p2platency.NewPeer(p2platency.WithID(""))
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Empty(t, p.GetGroup())
		require.NotEmpty(t, p.GetID())
	})

	t.Run("successfully creates new empty peer when given zeroed interval", func(t *testing.T) {
		t.Parallel()

		p, err := p2platency.NewPeer(p2platency.WithInterval(0))
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Equal(t, p2platency.DefaultInterval, p.GetInterval())
	})

	t.Run("does not panic when passed nil options", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() {
			_, err := p2platency.NewPeer(nil, nil)
			require.NoError(t, err)
		})
	})

	t.Run("executes provided options", func(t *testing.T) {
		t.Parallel()

		var called bool
		optOk := func(p *p2platency.Peer) error {
			called = true
			return nil
		}

		optFail := func(p *p2platency.Peer) error {
			return errors.New("error")
		}

		_, err := p2platency.NewPeer(optOk)
		require.NoError(t, err)
		require.True(t, called)

		_, err = p2platency.NewPeer(optFail)
		require.Error(t, err)
	})
}

func TestPeer_Start(t *testing.T) {
	t.Parallel()

	t.Run("successfully starts peer", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		mockRecorder := NewMockRecorder(gomock.NewController(t))

		ports, err := freeport.GetFreePorts(3)
		require.NoError(t, err)

		p, err := p2platency.NewPeer(
			p2platency.WithListenAddress(net.JoinHostPort("localhost", strconv.Itoa(ports[0]))),
			p2platency.WithSendAddresses(
				net.JoinHostPort("localhost", strconv.Itoa(ports[1])),
				net.JoinHostPort("localhost", strconv.Itoa(ports[2])),
			),
			p2platency.WithRecorder(mockRecorder),
			p2platency.WithInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		mockRecorder.EXPECT().Record(gomock.Any(), gomock.Any()).Do(func(context.Context, recorder.Sample) {
			cancel()
		}).AnyTimes()

		err = p.Start(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestPeer_Stop(t *testing.T) {
	t.Parallel()

	t.Run("successfully stops peer group", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		p, err := p2platency.NewPeer()
		require.NoError(t, err)

		stopped := false
		p.SetGroup(graceful.Group{
			graceful.RunnerType{
				StopFunc: func(context.Context) error {
					stopped = true
					return nil
				},
			},
		})

		p.CloseStoppedCh()

		err = p.Stop(ctx)
		require.NoError(t, err)
		require.True(t, stopped)
	})

	t.Run("returns error when context is closed", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		p, err := p2platency.NewPeer()
		require.NoError(t, err)

		p.SetGroup(graceful.Group{
			graceful.RunnerType{
				StopFunc: func(ctx context.Context) error {
					return ctx.Err()
				},
			},
		})

		err = p.Stop(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns error from group stop", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		p, err := p2platency.NewPeer()
		require.NoError(t, err)

		p.SetGroup(graceful.Group{
			graceful.RunnerType{
				StopFunc: func(context.Context) error {
					return errors.New("error")
				},
			},
		})

		p.CloseStoppedCh()

		err = p.Stop(ctx)
		require.Error(t, err)
	})
}
