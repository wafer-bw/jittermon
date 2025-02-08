package peer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/peer"
)

func TestPeerRequestBuffers_Jitter(t *testing.T) {
	t.Parallel()

	// TODO: update tests now that we are calculating jitter according to
	// https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1

	// t.Run("measures delayed jitter", func(t *testing.T) {
	// 	t.Parallel()

	// 	pid := peer.PeerID("1")
	// 	now := time.Now()
	// 	b := peer.PeerRequestBuffers{}
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now,
	// 		R: now.Add(52 * time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(1 * time.Second),
	// 		R: now.Add(1*time.Second + 51*time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(2 * time.Second),
	// 		R: now.Add(2*time.Second + 54*time.Millisecond),
	// 	})

	// 	jitter, ok := b.Jitter(pid)
	// 	require.True(t, ok)
	// 	require.Equal(t, 4*time.Millisecond, jitter)
	// })

	// t.Run("measures hastened jitter", func(t *testing.T) {
	// 	t.Parallel()

	// 	pid := peer.PeerID("1")
	// 	now := time.Now()
	// 	b := peer.PeerRequestBuffers{}
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now,
	// 		R: now.Add(52 * time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(1 * time.Second),
	// 		R: now.Add(1*time.Second + 53*time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(2 * time.Second),
	// 		R: now.Add(2*time.Second + 48*time.Millisecond),
	// 	})

	// 	jitter, ok := b.Jitter(pid)
	// 	require.True(t, ok)
	// 	require.Equal(t, 6*time.Millisecond, jitter)
	// })

	// t.Run("negates delayed send jitter from delayed receive jitter", func(t *testing.T) {
	// 	t.Parallel()

	// 	pid := peer.PeerID("1")
	// 	now := time.Now()
	// 	b := peer.PeerRequestBuffers{}
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now,
	// 		R: now.Add(52 * time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(1 * time.Second),
	// 		R: now.Add(1*time.Second + 51*time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(2*time.Second + 2*time.Millisecond),
	// 		R: now.Add(2*time.Second + 54*time.Millisecond),
	// 	})

	// 	jitter, ok := b.Jitter(pid)
	// 	require.True(t, ok)
	// 	require.Equal(t, 2*time.Millisecond, jitter)
	// })

	// t.Run("negates hastened send jitter from hastened receive jitter", func(t *testing.T) {
	// 	t.Parallel()

	// 	pid := peer.PeerID("1")
	// 	now := time.Now()
	// 	b := peer.PeerRequestBuffers{}
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now,
	// 		R: now.Add(50 * time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(1 * time.Second),
	// 		R: now.Add(1*time.Second + 50*time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(2*time.Second - 2*time.Millisecond),
	// 		R: now.Add(2*time.Second + 47*time.Millisecond),
	// 	})

	// 	jitter, ok := b.Jitter(pid)
	// 	require.True(t, ok)
	// 	require.Equal(t, 1*time.Millisecond, jitter)
	// })

	// t.Run("negates hastened send jitter from delayed receive jitter", func(t *testing.T) {
	// 	t.Parallel()

	// 	pid := peer.PeerID("1")
	// 	now := time.Now()
	// 	b := peer.PeerRequestBuffers{}
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now,
	// 		R: now.Add(50 * time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(1 * time.Second),
	// 		R: now.Add(1*time.Second + 50*time.Millisecond),
	// 	})
	// 	b.Sample(pid, peer.PeerRequest{
	// 		S: now.Add(2*time.Second - 2*time.Millisecond),
	// 		R: now.Add(2*time.Second + 54*time.Millisecond),
	// 	})

	// 	jitter, ok := b.Jitter(pid)
	// 	require.True(t, ok)
	// 	require.Equal(t, 6*time.Millisecond, jitter)
	// })

	t.Run("negates delayed send jitter from hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Sample(pid, peer.PeerRequest{
			S: now,
			R: now.Add(50 * time.Millisecond),
		})
		b.Sample(pid, peer.PeerRequest{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, peer.PeerRequest{
			S: now.Add(2*time.Second + 2*time.Millisecond),
			R: now.Add(2*time.Second + 46*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 375*time.Microsecond, jitter)
	})
}
