package p2platency

import (
	"github.com/wafer-bw/jittermon/internal/recorder"
	"github.com/wafer-bw/jittermon/internal/sampler/p2platency/internal/pollpb"
)

type Recorder interface {
	recorder.Recorder
}

type GRPCClientPoller interface {
	pollpb.PollServiceClient
}
