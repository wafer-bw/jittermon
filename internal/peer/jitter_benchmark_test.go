package peer_test

import (
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
