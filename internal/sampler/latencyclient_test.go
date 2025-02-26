package sampler_test

// import (
// 	"context"
// 	"log/slog"
// 	"net"
// 	"strconv"
// 	"sync"
// 	"testing"
// 	"time"

// 	"github.com/phayes/freeport"
// 	"github.com/stretchr/testify/require"
// 	"github.com/wafer-bw/go-toolbox/always"
// 	"github.com/wafer-bw/jittermon/internal/recorder"
// 	"github.com/wafer-bw/jittermon/internal/sampler"
// )

// func TestLatencyClient(t *testing.T) {
// 	t.Parallel()

// 	t.Run("starts & stops the client, calling the poll method at provided interval", func(t *testing.T) {
// 		t.Parallel()

// 		ctx := t.Context()
// 		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

// 		opts := sampler.LatencyClientOptions{
// 			ID:       "test",
// 			Address:  addr,
// 			Interval: 10 * time.Millisecond,
// 			Recorder: recorder.NoOp(),
// 			Log:      slog.New(slog.DiscardHandler),
// 		}

// 		c, err := sampler.NewLatencyClient(opts)
// 		require.NoError(t, err)

// 		mockPoller := &poller{mu: &sync.RWMutex{}}
// 		c.SetClient(mockPoller)
// 		require.NotNil(t, c.GetClient())

// 		go func() {
// 			err := c.Start(ctx)
// 			require.NoError(t, err)
// 		}()

// 		require.Eventually(t, func() bool {
// 			return mockPoller.GetCalled()
// 		}, 1*time.Second, 25*time.Millisecond)

// 		err = c.Stop(ctx)
// 		require.NoError(t, err)
// 	})

// 	t.Run("starts & stops the client non-gracefully if context is over", func(t *testing.T) {
// 		t.Parallel()

// 		ctx, cancel := context.WithCancel(t.Context())
// 		addr := net.JoinHostPort("", strconv.Itoa(always.Accept(freeport.GetFreePort())))

// 		c, err := sampler.NewLatencyClient(addr, &doPoller{mu: &sync.RWMutex{}}, 10*time.Millisecond, slog.New(slog.DiscardHandler))
// 		require.NoError(t, err)

// 		go func() {
// 			err := c.Start(ctx)
// 			require.ErrorIs(t, err, context.Canceled)
// 		}()

// 		require.Eventually(t, func() bool {
// 			p, ok := c.GetPoller().(*doPoller)
// 			require.True(t, ok)
// 			cancel()
// 			return p.GetCalled()
// 		}, 1*time.Second, 25*time.Millisecond)

// 		err = c.Stop(ctx)
// 		require.ErrorIs(t, err, context.Canceled)
// 	})
// }
