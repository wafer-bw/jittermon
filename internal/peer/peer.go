package peer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/net"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	downstreamJitterKey string = "downstreamJitter"
	upstreamJitterKey   string = "upstreamJitter"
	rttKey              string = "rtt"
)

var _ pollpb.PollServiceServer = (*Peer)(nil)
var _ net.DoPoller = (*Peer)(nil)

// Recorder is capable of persisting a keyed duration value for an interaction
// between two peers in some storage media or mechanism.
//
// src: source peer ID.
// dst: destination peer ID.
// key: identifies the duration value's meaning.
// tsm: observation timestamp of the interaction.
// dur: duration value (e.g. jitter, rtt, etc).
//
// TODO: expand out var names probably.
// TODO: maybe make src/dst be local/remote instead?
type Recorder interface {
	Record(src, dst PeerID, key string, tsm time.Time, dur time.Duration) error
}

type PeerID string

func (p PeerID) String() string {
	return string(p)
}

type Peer struct {
	id             PeerID
	log            *slog.Logger
	jitter         Recorder
	rtt            Recorder
	requestBuffers PeerRequestBuffers

	pollpb.UnimplementedPollServiceServer
}

// TODO: switch to functional options or options struct?
func NewPeer(ID string, jitRecorder, rttRecorder Recorder, log *slog.Logger) (*Peer, error) {
	return &Peer{
		id:             PeerID(ID),
		log:            log,
		jitter:         jitRecorder,
		rtt:            rttRecorder,
		requestBuffers: PeerRequestBuffers{},
	}, nil
}

// Poll handles incoming poll requests.
func (p *Peer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		p.log.Error("poll request with no timestamp")
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	peerIDPb := req.GetId()
	if peerIDPb == "" {
		p.log.Error("poll request with no peer ID")
		return nil, fmt.Errorf("peer ID is required")
	}
	peerID := PeerID(peerIDPb)

	p.requestBuffers.Sample(peerID, PeerRequest{SentAt: sentAt, ReceivedAt: now})

	resp := &pollpb.PollResponse{}
	resp.SetId(p.id.String())

	jitter, ok := p.requestBuffers.Jitter(peerID)
	if !ok {
		return resp, nil
	}

	p.log.Debug("", "src", peerID, "dst", p.id, downstreamJitterKey, jitter)

	resp.SetJitter(durationpb.New(jitter))

	if p.jitter != nil {
		// record downstream jitter relative to this peer.
		if err := p.jitter.Record(peerID, p.id, downstreamJitterKey, now, jitter); err != nil {
			p.log.Error("failed to record downstream jitter", "err", err)
		}
	}

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient) error {
	// TODO: may want to accept addr as an argument here for easily identifying
	// a failing peer in error messages.

	now := time.Now()
	rtt := time.Since(now)

	req := &pollpb.PollRequest{}
	req.SetId(p.id.String())
	req.SetTimestamp(timestamppb.New(now))

	resp, err := client.Poll(ctx, req)
	if err != nil {
		p.log.Warn("poll failed", "err", err)
		return err
	}

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jitter := jitterPb.AsDuration()

	peerIDPb := resp.GetId()
	if peerIDPb == "" {
		return fmt.Errorf("no peer ID in request")
	}
	peerID := PeerID(peerIDPb)

	if p.jitter != nil {
		// record upstream jitter relative to this peer.
		if err := p.jitter.Record(p.id, peerID, upstreamJitterKey, now, jitter); err != nil {
			p.log.Error("failed to record upstream jitter", "err", err)
		}
	}

	if p.rtt != nil {
		// record round-trip time between this peer and the remote peer.
		if err := p.rtt.Record(p.id, peerID, rttKey, now, rtt); err != nil {
			p.log.Error("failed to record rtt", "err", err)
		}
	}

	p.log.Debug("", "src", p.id, "dst", peerID, rttKey, rtt)
	p.log.Debug("", "src", p.id, "dst", peerID, upstreamJitterKey, jitter)

	return nil
}
