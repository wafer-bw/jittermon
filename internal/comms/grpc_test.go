package comms_test

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/go-toolbox/always"
	"github.com/wafer-bw/jittermon/internal/comms"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
)

type doPoller struct {
	mu     *sync.RWMutex
	called bool
}

func (d *doPoller) DoPoll(ctx context.Context, client pollpb.PollServiceClient, addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.called = true
	return nil
}

func (d *doPoller) DoTrace(ctx context.Context, addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.called = true
	return nil
}

func (d *doPoller) GetCalled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.called
}

func TestServer_StartStop(t *testing.T) {
	t.Parallel()

	t.Run("starts & stops the server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

		s, err := comms.NewServer(addr, &pollpb.UnimplementedPollServiceServer{}, slog.New(slog.DiscardHandler))
		require.NoError(t, err)

		go func() {
			err := s.Start(ctx)
			require.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return false
			}
			conn.Close()
			return true
		}, 1*time.Second, 25*time.Millisecond)

		err = s.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("starts & stops the server non-gracefully if context is over", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

		s, err := comms.NewServer(addr, &pollpb.UnimplementedPollServiceServer{}, slog.New(slog.DiscardHandler))
		require.NoError(t, err)

		go func() {
			err := s.Start(ctx)
			require.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return false
			}
			conn.Close()
			cancel()
			return true
		}, 1*time.Second, 25*time.Millisecond)

		err = s.Stop(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns error if unable to start server", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		s, err := comms.NewServer("-1", &pollpb.UnimplementedPollServiceServer{}, slog.New(slog.DiscardHandler))
		require.NoError(t, err)

		err = s.Start(ctx)
		require.Error(t, err)
	})
}

func TestClient_StartStop(t *testing.T) {
	t.Parallel()

	t.Run("starts & stops the client, calling the do poll method", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

		c, err := comms.NewClient(addr, &doPoller{mu: &sync.RWMutex{}}, 10*time.Millisecond, slog.New(slog.DiscardHandler))
		require.NoError(t, err)

		go func() {
			err := c.Start(ctx)
			require.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			p, ok := c.GetPoller().(*doPoller)
			require.True(t, ok)
			return p.GetCalled()
		}, 1*time.Second, 25*time.Millisecond)

		err = c.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("starts & stops the client non-gracefully if context is over", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

		c, err := comms.NewClient(addr, &doPoller{mu: &sync.RWMutex{}}, 10*time.Millisecond, slog.New(slog.DiscardHandler))
		require.NoError(t, err)

		go func() {
			err := c.Start(ctx)
			require.ErrorIs(t, err, context.Canceled)
		}()

		require.Eventually(t, func() bool {
			p, ok := c.GetPoller().(*doPoller)
			require.True(t, ok)
			cancel()
			return p.GetCalled()
		}, 1*time.Second, 25*time.Millisecond)

		err = c.Stop(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})
}
