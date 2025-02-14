package peer

import (
	"context"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/recorder"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

// export for mocking.
type Recorder interface {
	recorder.Recorder
}

// export for mocking.
type PollServiceClient interface {
	pollpb.PollServiceClient
}

// export for testing.
func (p Peer) GetLogger() *slog.Logger {
	return p.log
}

// export for testing.
func (p Peer) GetRecorder() recorder.Recorder {
	return p.r
}

// export for testing.
func (p Peer) GetID() string {
	return p.id
}

// export for benchmarking.
type StubPeerClient struct {
	ID string
}

// export for benchmarking.
func (s StubPeerClient) Poll(ctx context.Context, in *pollpb.PollRequest, opts ...grpc.CallOption) (*pollpb.PollResponse, error) {
	resp := &pollpb.PollResponse{}
	resp.SetId("id")
	resp.SetJitter(durationpb.New(1 * time.Millisecond))

	return &pollpb.PollResponse{}, nil
}
