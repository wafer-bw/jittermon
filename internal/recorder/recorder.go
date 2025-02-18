package recorder

import (
	"context"
	"time"
)

// TODO: docstring.
type SampleType string

const (
	SampleTypeDownstreamJitter SampleType = "downstream_jitter"
	SampleTypeUpstreamJitter   SampleType = "upstream_jitter"
	SampleTypeSentPackets      SampleType = "sent_packets"
	SampleTypeLostPackets      SampleType = "lost_packets"
	SampleTypeRTT              SampleType = "rtt" // round trip time (ping).
)

// Sample is the data structure that is recorded by a [Recorder].
type Sample struct {
	Time time.Time
	Type SampleType
	Src  string // source address/id/name.
	Dst  string // destination address/id/name.
	Val  any    // value to record if there is one.
}

// ChainLink is a function that accepts and returns a [Recorder]. The returned
// [Recorder] should call `next.Record()` to continue the chain.
type ChainLink func(next Recorder) Recorder

// Recorder is capable of recording the provided [Sample] and should respect
// the lifetime of the provided [context.Context].
type Recorder interface {
	Record(context.Context, Sample)
}

// RecorderFunc is an adapter to allow the use of ordinary functions as
// recorders. If f is a function with the appropriate signature, RecorderFunc(f)
// is a [Recorder] that calls f.
type RecorderFunc func(context.Context, Sample)

// Record calls f(ctx, s).
func (f RecorderFunc) Record(ctx context.Context, s Sample) {
	f(ctx, s)
}

// Chain [ChainLink]s together to create a single [Recorder].
func Chain(recorders ...ChainLink) Recorder {
	terminal := RecorderFunc(func(ctx context.Context, s Sample) { return })
	if len(recorders) == 0 {
		return terminal
	}

	r := recorders[len(recorders)-1](terminal)
	//nolint:mnd // link second last to last (r) on first iteration.
	for i := len(recorders) - 2; i >= 0; i-- {
		r = recorders[i](r)
	}

	return r
}
