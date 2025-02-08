package peer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/peer"
)

func TestPeerRequestBuffers_Jitter(t *testing.T) {
	t.Parallel()

	t.Run("measures delayed jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(52 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 51*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2 * time.Second),
			ReceivedAt: now.Add(2*time.Second + 54*time.Millisecond),
		})

		// Send Jitter: 0
		// Receive Jitter:
		// 51 - 52 = -1
		// 54 - 51 = 3
		// 3 - -1 = 4 (4ms delayed)

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 4*time.Millisecond, jitter)
	})

	t.Run("measures hastened jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(52 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 53*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2 * time.Second),
			ReceivedAt: now.Add(2*time.Second + 48*time.Millisecond),
		})

		// Send Jitter: 0
		// Receive Jitter:
		// 53 - 52 = 1
		// 48 - 53 = -5
		// -5 - 1 = -6 (6ms hastened)

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 6*time.Millisecond, jitter)
	})

	t.Run("negates delayed send jitter from delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(52 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 51*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2*time.Second + 2*time.Millisecond),
			ReceivedAt: now.Add(2*time.Second + 54*time.Millisecond),
		})

		// Send Jitter:
		// 1.000 - 0.000 = 1.000
		// 2.002 - 1.000 = 1.002
		// 1.002 - 1.000 = 0.002 (2ms delayed)
		//
		// Receive Jitter:
		// 51 - 52 = -1
		// 54 - 51 = 3
		// 3 - -1 = 4 (4ms delayed)
		//
		// Negated:
		// 4 - 2 = 2

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 2*time.Millisecond, jitter)
	})

	t.Run("negates hastened send jitter from hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(50 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2*time.Second - 2*time.Millisecond),
			ReceivedAt: now.Add(2*time.Second + 47*time.Millisecond),
		})

		// Send Jitter:
		// 1.000 - 0.000 = 1.000
		// 1.998 - 1.000 = 0.998
		// 0.998 - 1.000 = -0.002 (2ms hastened)
		//
		// Receive Jitter:
		// 50 - 50 = 0
		// 47 - 50 = -3 (3ms hastened)
		//
		// Negated:
		// -3 - -2 = |-1| -> 1

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 1*time.Millisecond, jitter)
	})

	t.Run("negates hastened send jitter from delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(50 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2*time.Second - 2*time.Millisecond),
			ReceivedAt: now.Add(2*time.Second + 54*time.Millisecond),
		})

		// Send Jitter:
		// 1.000 - 0.000 = 1.000
		// 1.998 - 1.000 = 0.998
		// 0.998 - 1.000 = -0.002 (2ms hastened)
		//
		// Receive Jitter:
		// 50 - 50 = 0
		// 54 - 50 = 4
		// 4 - 0 = 4 (4ms delayed)
		//
		// Negated:
		// 4 - -2 = 6 (6ms delayed)
		//
		// If there was no send/receive jitter we would expect the 3rd packet to
		// be received at 2.050, since the send came 2ms early we can adjust our
		// expectatations such that it would be received at 2.048, but it
		// actually came at 2.054, so it was 6ms delayed.

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 6*time.Millisecond, jitter)
	})

	t.Run("negates delayed send jitter from hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := peer.PeerID("1")
		now := time.Now()
		b := peer.PeerRequestBuffers{}
		b.Add(pid, peer.PeerRequest{
			SentAt:     now,
			ReceivedAt: now.Add(50 * time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(1 * time.Second),
			ReceivedAt: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Add(pid, peer.PeerRequest{
			SentAt:     now.Add(2*time.Second + 2*time.Millisecond),
			ReceivedAt: now.Add(2*time.Second + 46*time.Millisecond),
		})

		// Send Jitter:
		// 1.000 - 0.000 = 1.000
		// 2.002 - 1.000 = 1.002
		// 1.002 - 1.000 = 0.002 (2ms delayed)
		//
		// Receive Jitter:
		// 50 - 50 = 0
		// 46 - 50 = -4
		// -4 - 0 = -4 (4ms hastened)
		//
		// Negated:
		// -4 - 2 =  -6 (6ms hastened)
		//
		// If there was no send/receive jitter we would expect the 3rd packet to
		// be received at 2.050, since the send came 2ms late we can adjust our
		// expectatations such that it would be received at 2.052, but it
		// actually came at 2.046, so it was 6ms hastened.

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 6*time.Millisecond, jitter)
	})
}
