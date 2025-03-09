package recorder

import (
	"context"
	"time"
)

// NoOp recorder for when you don't want samples to be recorded.
var NoOp RecorderFunc = func(context.Context, Sample) {}

// Labels provides a deterministicly ordered set of key-value pair labels for
// use with [Sample].
type Labels []Label

func (ls Labels) Keys() []string {
	keys := make([]string, len(ls))
	for i, l := range ls {
		keys[i] = l.K
	}
	return keys
}

func (ls Labels) Values() []string {
	values := make([]string, len(ls))
	for i, l := range ls {
		values[i] = l.V
	}
	return values
}

type Label struct {
	K string
	V string
}

// TODO: docstring.
type SampleType string

const (
	SampleTypeDownstreamJitter SampleType = "downstream_jitter"
	SampleTypeUpstreamJitter   SampleType = "upstream_jitter"
	SampleTypeSentPackets      SampleType = "sent_packets"
	SampleTypeLostPackets      SampleType = "lost_packets"
	SampleTypeRTT              SampleType = "rtt"     // round trip time (ping).
	SampleTypeHopRTT           SampleType = "hop_rtt" // traceroute hop rtt.
)

// Sample is the data structure that is recorded by a [Recorder].
type Sample struct {
	Time   time.Time
	Type   SampleType // TODO: rename to Key and likely just make it a string.
	Val    any        // value to record if there is one.
	Labels Labels
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
	// TODO: consider making receiver of type []ChainLink.
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
