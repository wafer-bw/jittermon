package recorder

import (
	"context"
	"time"
)

// TODO: docstring.
type SampleType string

const (
	// TODO: docstring.
	SampleTypeDownstreamJitter SampleType = "downstream_jitter"

	// TODO: docstring.
	SampleTypeUpstreamJitter SampleType = "upstream_jitter"

	// TODO: docstring.
	SampleTypeSentPackets SampleType = "sent_packets"

	// TODO: docstring.
	SampleTypeLostPackets SampleType = "lost_packets"

	// TODO: docstring.
	SampleTypeRTT SampleType = "rtt"
)

// TODO: docstring.
type Sample struct {
	Time time.Time
	Type SampleType
	Src  string
	Dst  string
	Val  any
}

// TODO: docstring.
type Recorder interface {
	Record(context.Context, Sample)
}

// TODO: docstring.
type RecorderFunc func(context.Context, Sample)

// TODO: docstring.
func (f RecorderFunc) Record(ctx context.Context, s Sample) {
	f(ctx, s)
}

// TODO: docstring.
func Chain(recorders ...func(Recorder) Recorder) Recorder {
	terminal := RecorderFunc(func(ctx context.Context, s Sample) { return })
	if len(recorders) == 0 {
		return terminal
	}

	r := recorders[len(recorders)-1](terminal)
	for i := len(recorders) - 1; i >= 0; i-- {
		r = recorders[i](r)
	}

	return r
}
