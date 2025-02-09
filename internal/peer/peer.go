package peer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/comms"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	downstreamJitterKey string = "downstream_jitter"
	upstreamJitterKey   string = "upstream_jitter"
	sentPacketsKey      string = "sent_packets"
	lostPacketsKey      string = "lost_packets"
	rttKey              string = "rtt"
)

var _ pollpb.PollServiceServer = (*Peer)(nil)
var _ comms.DoPoller = (*Peer)(nil)

// Recorder is capable of persisting a keyed duration value for an interaction
// between two peers in some storage media or mechanism.
//
// src: source peer address.
// dst: destination peer address.
// key: identifies the duration value's meaning.
// tsm: observation timestamp of the interaction.
// dur: duration value (e.g. jitter, rtt, etc).
//
// TODO: expand out var names probably.
// TODO: maybe make src/dst be local/remote instead?
type Recorder interface {
	Record(tsm time.Time, key, src, dst string, dur *time.Duration) error
}

// TODO: a better way of specifying which recorders to use and which metrics
// to record to each recorder.
type Peer struct {
	id             string
	log            *slog.Logger
	jitter         Recorder
	rtt            Recorder
	pl             Recorder
	requestBuffers jitter.HostPacketBuffers

	pollpb.UnimplementedPollServiceServer
}

// TODO: switch to functional options or options struct?
func NewPeer(id string, jitRecorder, rttRecorder, plRecorder Recorder, log *slog.Logger) (*Peer, error) {
	return &Peer{
		id:             id,
		log:            log,
		jitter:         jitRecorder,
		rtt:            rttRecorder,
		pl:             plRecorder,
		requestBuffers: jitter.NewHostPacketBuffers(),
	}, nil
}

// Poll handles incoming poll requests.
func (p *Peer) Poll(ctx context.Context, req *pollpb.PollRequest) (*pollpb.PollResponse, error) {
	now := time.Now()
	resp := &pollpb.PollResponse{}
	resp.SetId(p.id)

	srcID := req.GetId()
	if srcID == "" {
		return nil, fmt.Errorf("id is required")
	}

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	p.requestBuffers.Sample(srcID, jitter.Packet{S: sentAt, R: now})
	jitter, ok := p.requestBuffers.Jitter(srcID)
	if !ok {
		return resp, nil
	}

	resp.SetJitter(durationpb.New(jitter))

	if p.jitter != nil {
		if err := p.jitter.Record(now, downstreamJitterKey, srcID, p.id, &jitter); err != nil {
			p.log.Error("failed to record downstream jitter", "err", err)
		}
	}

	p.log.Debug("", "src", srcID, "dst", p.id, downstreamJitterKey, jitter)

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient, dstAddr string) error {
	now := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(p.id)
	req.SetTimestamp(timestamppb.New(now))

	if p.pl != nil {
		if err := p.pl.Record(now, sentPacketsKey, p.id, dstAddr, nil); err != nil {
			p.log.Error("failed to record sent packet", "err", err)
		}
	}

	resp, err := client.Poll(ctx, req)
	if err != nil {
		if p.pl != nil {
			if err := p.pl.Record(now, lostPacketsKey, p.id, dstAddr, nil); err != nil {
				p.log.Error("failed to record lost packet", "err", err)
			}
		}
		p.log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(now)

	dstID := resp.GetId()
	if dstID == "" {
		p.log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		p.log.Error("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jitter := jitterPb.AsDuration()

	if p.jitter != nil {
		if err := p.jitter.Record(now, upstreamJitterKey, p.id, dstID, &jitter); err != nil {
			p.log.Error("failed to record upstream jitter", "err", err)
		}
	}

	if p.rtt != nil {
		if err := p.rtt.Record(now, rttKey, p.id, dstID, &rtt); err != nil {
			p.log.Error("failed to record rtt", "err", err)
		}
	}

	p.log.Debug("", "src", p.id, "dst", dstID, rttKey, rtt)
	p.log.Debug("", "src", p.id, "dst", dstID, upstreamJitterKey, jitter)

	return nil
}
