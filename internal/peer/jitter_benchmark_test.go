package peer_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/peer"
)

func BenchmarkPeerRequestBuffers_Jitter(b *testing.B) {
	now := time.Now()
	buffers := peer.PeerRequestBuffers{}

	buffers.Sample("peer", peer.PeerRequest{S: now, R: now.Add(52 * time.Millisecond)})
	buffers.Sample("peer", peer.PeerRequest{S: now.Add(52 * time.Millisecond), R: now.Add(100 * time.Millisecond)})
	buffers.Sample("peer", peer.PeerRequest{S: now.Add(100 * time.Millisecond), R: now.Add(153 * time.Millisecond)})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Jitter("peer")
	}
}

func BenchmarkPeerRequestBuffers_Sample(b *testing.B) {
	now := time.Now()
	buffers := peer.PeerRequestBuffers{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Sample(peer.PeerID(fmt.Sprintf("peer%d", i)), peer.PeerRequest{S: now, R: now})
	}
}
