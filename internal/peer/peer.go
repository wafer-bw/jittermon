package peer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ pollpb.PollServiceServer = (*Peer)(nil)
var _ DoPoller = (*Peer)(nil)

type Peer struct {
	id  string
	log *slog.Logger

	// TODO: record upstream & downstream jitter into a recorder interface.
	requestBuffers PeerRequestBuffers

	pollpb.UnimplementedPollServiceServer
}

func NewPeer(ID string, log *slog.Logger) (*Peer, error) {
	return &Peer{
		id:             ID,
		log:            log,
		requestBuffers: PeerRequestBuffers{},
	}, nil
}

// Poll handles incoming poll requests.
func (p *Peer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	receivedAt := time.Now()

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		p.log.Error("poll request with no timestamp")
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	peerID := req.GetId()
	if peerID == "" {
		p.log.Error("poll request with no peer ID")
		return nil, fmt.Errorf("peer ID is required")
	}

	p.requestBuffers.Add(peerID, PeerRequest{sentAt: sentAt, receivedAt: receivedAt})

	resp := &pollpb.PollResponse{}
	resp.SetId(p.id)

	jitter, ok := p.requestBuffers.Jitter(peerID) // downstream jitter to this peer.
	if ok {
		p.log.Info(jitter.String(), "id", p.id, "peerId", peerID)
		resp.SetJitter(durationpb.New(jitter))
	}

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient) error {
	// TODO: may want to accept addr as an argument here for easily identifying
	// a failing peer in error messages.

	s := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(p.id)
	req.SetTimestamp(timestamppb.New(s))

	resp, err := client.Poll(ctx, req)
	if err != nil {
		p.log.Warn("poll failed", "err", err)
		return err
	}
	rtt := time.Since(s)

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jitter := jitterPb.AsDuration() // upstream jitter from this peer.

	p.log.Info("", "rtt", rtt, "jitter", jitter)

	return nil
}
