package jitter_test

import (
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
)

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/jitter
// cpu: Apple M2 Max
// BenchmarkBuffer_Interarrival-12    	17989352	        63.13 ns/op	     112 B/op	       0 allocs/op

func BenchmarkBuffer_Interarrival(b *testing.B) {
	pid := "peer"
	now := time.Now()
	now.Add(25 * time.Millisecond)
	buffer := jitter.Buffer{}

	for b.Loop() {
		buffer.Interarrival(pid, now, now)
	}
}
