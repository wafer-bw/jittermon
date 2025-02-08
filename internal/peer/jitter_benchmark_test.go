package peer_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/peer"
)

func BenchmarkRequestBuffer_Jitter(b *testing.B) {
	now := time.Now()
	buffer := peer.RequestBuffer{
		{SentAt: now, ReceivedAt: now.Add(52 * time.Millisecond)},
		{SentAt: now.Add(52 * time.Millisecond), ReceivedAt: now.Add(100 * time.Millisecond)},
		{SentAt: now.Add(100 * time.Millisecond), ReceivedAt: now.Add(153 * time.Millisecond)},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Jitter()
	}
}

func BenchmarkRequestBuffer_Push(b *testing.B) {
	now := time.Now()
	buffer := peer.RequestBuffer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Push(peer.PeerRequest{SentAt: now, ReceivedAt: now})
	}
}

func BenchmarkPeerRequestBuffers_Jitter(b *testing.B) {
	now := time.Now()
	buffers := peer.PeerRequestBuffers{}
	for i := range 50 {
		buffers.Add(peer.PeerID(fmt.Sprintf("peer%d", i)), peer.PeerRequest{SentAt: now, ReceivedAt: now})
	}
	buffers.Add("peer0", peer.PeerRequest{SentAt: now, ReceivedAt: now.Add(52 * time.Millisecond)})
	buffers.Add("peer0", peer.PeerRequest{SentAt: now.Add(52 * time.Millisecond), ReceivedAt: now.Add(100 * time.Millisecond)})
	buffers.Add("peer0", peer.PeerRequest{SentAt: now.Add(100 * time.Millisecond), ReceivedAt: now.Add(153 * time.Millisecond)})
	for i := range 50 {
		buffers.Add(peer.PeerID(fmt.Sprintf("peer%d", i+51)), peer.PeerRequest{SentAt: now, ReceivedAt: now})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Jitter("peer1")
	}
}

func BenchmarkPeerRequestBuffers_Add(b *testing.B) {
	now := time.Now()
	buffers := peer.PeerRequestBuffers{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Add(peer.PeerID(fmt.Sprintf("peer%d", i)), peer.PeerRequest{SentAt: now, ReceivedAt: now})
	}
}
