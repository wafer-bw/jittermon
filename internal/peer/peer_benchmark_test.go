package peer_test

// goos: darwin
// goarch: arm64
// pkg: github.com/wafer-bw/jittermon/internal/peer
// cpu: Apple M2 Max
// BenchmarkPoll-12      	 4556762	       257.1 ns/op	     344 B/op	       6 allocs/op
// BenchmarkDoPoll-12    	 4349839	       279.1 ns/op	     424 B/op	      11 allocs/op

import (
	"context"
	"testing"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func BenchmarkPoll(b *testing.B) {
	p, err := peer.NewPeer()
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		req := &pollpb.PollRequest{}
		req.SetId("id")
		req.SetTimestamp(timestamppb.Now())

		p.Poll(context.Background(), req)
	}
}

func BenchmarkDoPoll(b *testing.B) {
	p, err := peer.NewPeer()
	if err != nil {
		b.Fatal(err)
	}

	client := peer.StubPeerClient{ID: "id"}

	for b.Loop() {
		req := &pollpb.PollRequest{}
		req.SetId("id")
		req.SetTimestamp(timestamppb.Now())

		p.DoPoll(context.Background(), client, "localhost")
	}
}
