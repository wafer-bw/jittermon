package recorder_test

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/recorder
// cpu: Apple M2 Max
// BenchmarkPrometheus_RecordDuration/with_no_labels-12         	16667176	        74.23 ns/op	      32 B/op	       1 allocs/op
// BenchmarkPrometheus_RecordDuration/with_2_labels-12          	11948625	        98.00 ns/op	      64 B/op	       2 allocs/op
// BenchmarkPrometheus_RecordIncrement/with_no_labels-12        	19204582	        63.71 ns/op	      32 B/op	       1 allocs/op
// BenchmarkPrometheus_RecordIncrement/with_2_labels-12         	13772055	        86.11 ns/op	      64 B/op	       2 allocs/op

import (
	"context"
	"testing"
	"time"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

func BenchmarkPrometheus_RecordDuration(b *testing.B) {
	ctx := b.Context()

	p, err := recorder.NewPrometheus(":8080", nil)
	if err != nil {
		b.Fatal(err)
	}

	noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})

	b.Run("with no labels", func(b *testing.B) {
		sample := recorder.Sample{Type: "abc", Val: 1 * time.Second}
		for b.Loop() {
			p.RecordDuration(noop).Record(ctx, sample)
		}
	})

	b.Run("with 2 labels", func(b *testing.B) {
		labels := recorder.Labels{recorder.Label{K: "a", V: "b"}, recorder.Label{K: "c", V: "d"}}
		sample := recorder.Sample{Type: "def", Val: 1 * time.Second, Labels: labels}
		for b.Loop() {
			p.RecordDuration(noop).Record(ctx, sample)
		}
	})
}

func BenchmarkPrometheus_RecordIncrement(b *testing.B) {
	ctx := b.Context()

	p, err := recorder.NewPrometheus(":8080", nil)
	if err != nil {
		b.Fatal(err)
	}

	noop := recorder.RecorderFunc(func(context.Context, recorder.Sample) {})

	b.Run("with no labels", func(b *testing.B) {
		sample := recorder.Sample{Type: "abc", Val: struct{}{}}
		for b.Loop() {
			p.RecordIncrement(noop).Record(ctx, sample)
		}
	})

	b.Run("with 2 labels", func(b *testing.B) {
		labels := recorder.Labels{recorder.Label{K: "a", V: "b"}, recorder.Label{K: "c", V: "d"}}
		sample := recorder.Sample{Type: "def", Val: struct{}{}, Labels: labels}
		for b.Loop() {
			p.RecordIncrement(noop).Record(ctx, sample)
		}
	})
}
