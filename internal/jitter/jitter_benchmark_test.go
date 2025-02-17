package jitter_test

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/jitter
// cpu: Apple M2 Max
// BenchmarkHostPacketBuffers_Jitter-12    	42102183	        27.68 ns/op	       0 B/op	       0 allocs/op
// BenchmarkHostPacketBuffers_Sample-12    	22092580	        51.34 ns/op	     112 B/op	       0 allocs/op

import (
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
)

func BenchmarkHostPacketBuffers_Jitter(b *testing.B) {
	now := time.Now()
	buffers := jitter.NewHostPacketBuffers()

	buffers.Sample("peer", jitter.Packet{S: now.Add(52 * time.Millisecond), R: now.Add(100 * time.Millisecond)})
	buffers.Sample("peer", jitter.Packet{S: now.Add(100 * time.Millisecond), R: now.Add(153 * time.Millisecond)})

	for b.Loop() {
		buffers.Jitter("peer")
	}
}

func BenchmarkHostPacketBuffers_Sample(b *testing.B) {
	now := time.Now()
	buffers := jitter.NewHostPacketBuffers()

	for b.Loop() {
		buffers.Sample("peer", jitter.Packet{S: now, R: now})
	}
}
