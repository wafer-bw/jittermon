package recorder_test

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/recorder
// cpu: Apple M2 Max
// BenchmarkPrometheus_RecordDuration-12     	14746377	        81.02 ns/op	      32 B/op	       1 allocs/op
// BenchmarkPrometheus_RecordIncrement-12    	18003543	        67.96 ns/op	      32 B/op	       1 allocs/op

import (
	"context"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

func BenchmarkPrometheus_RecordDuration(b *testing.B) {
	ctx := b.Context()
	sample := recorder.Sample{Type: "abc", Src: "a", Dst: "b", Val: 1 * time.Second}

	p, err := recorder.NewPrometheus(":8080")
	if err != nil {
		b.Fatal(err)
	}

	noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})

	for b.Loop() {
		p.RecordDuration(noop).Record(ctx, sample)
	}
}

func BenchmarkPrometheus_RecordIncrement(b *testing.B) {
	ctx := b.Context()
	sample := recorder.Sample{Type: "abc", Src: "a", Dst: "b", Val: struct{}{}}

	p, err := recorder.NewPrometheus(":8080")
	if err != nil {
		b.Fatal(err)
	}

	noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})

	for b.Loop() {
		p.RecordIncrement(noop).Record(ctx, sample)
	}
}
