package jitter_test

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/jitter
// cpu: Apple M2 Max
// BenchmarkPeerRequestBuffers_Jitter-12    	52776152	        22.05 ns/op	       0 B/op	       0 allocs/op
// BenchmarkPeerRequestBuffers_Sample-12    	 3575097	       378.6 ns/op	     312 B/op	       3 allocs/op

import (
	"fmt"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
)

func BenchmarkPeerRequestBuffers_Jitter(b *testing.B) {
	now := time.Now()
	buffers := jitter.NewHostPacketBuffers()

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
	buffers := jitter.NewHostPacketBuffers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers.Sample(fmt.Sprintf("peer%d", i), jitter.Packet{S: now, R: now})
	}
}
