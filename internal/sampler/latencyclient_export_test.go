package sampler

import (
	"fmt"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
)

// export for testing.
type PollServiceClient interface {
	pollpb.PollServiceClient
}

// export for testing.
func (lc *LatencyClient) SetClient(client pollpb.PollServiceClient) {
	lc.client = client
	fmt.Println("client set")
}
