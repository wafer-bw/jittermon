package jitter_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
)

func BenchmarkPeerRequestBuffers_Jitter(b *testing.B) {
	now := time.Now()
	buffers := jitter.HostPacketBuffers{}

	buffers.Sample("peer", jitter.Packet{S: now, R: now.Add(52 * time.Millisecond)})
	buffers.Sample("peer", jitter.Packet{S: now.Add(52 * time.Millisecond), R: now.Add(100 * time.Millisecond)})
	buffers.Sample("peer", jitter.Packet{S: now.Add(100 * time.Millisecond), R: now.Add(153 * time.Millisecond)})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Jitter("peer")
	}
}

func BenchmarkPeerRequestBuffers_Sample(b *testing.B) {
	now := time.Now()
	buffers := jitter.HostPacketBuffers{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Sample(fmt.Sprintf("peer%d", i), jitter.Packet{S: now, R: now})
	}
}
