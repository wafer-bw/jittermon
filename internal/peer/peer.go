package peer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/comms"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	downstreamJitterKey string = "downstreamJitter"
	upstreamJitterKey   string = "upstreamJitter"
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
	Record(src, dst string, key string, tsm time.Time, dur *time.Duration) error
}

// TODO: a better way of specifying which recorders to use and which metrics
// to record to each recorder.
type Peer struct {
	listenAddr     string
	log            *slog.Logger
	jitter         Recorder
	rtt            Recorder
	pl             Recorder
	requestBuffers jitter.HostPacketBuffers

	pollpb.UnimplementedPollServiceServer
}

// TODO: switch to functional options or options struct?
func NewPeer(listenAddr string, jitRecorder, rttRecorder, plRecorder Recorder, log *slog.Logger) (*Peer, error) {
	return &Peer{
		listenAddr:     listenAddr,
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

	peer, _ := peer.FromContext(ctx)
	peerAddr := peer.Addr.String()

	sentAtPb := req.GetTimestamp()
	if sentAtPb == nil {
		p.log.Error("poll request with no timestamp")
		return nil, fmt.Errorf("timestamp is required")
	}
	sentAt := sentAtPb.AsTime()

	p.requestBuffers.Sample(peerAddr, jitter.Packet{S: sentAt, R: now})
	jitter, ok := p.requestBuffers.Jitter(peerAddr)
	if !ok {
		return resp, nil
	}

	resp.SetJitter(durationpb.New(jitter))

	if p.jitter != nil {
		if err := p.jitter.Record(peerAddr, p.listenAddr, downstreamJitterKey, now, &jitter); err != nil {
			p.log.Error("failed to record downstream jitter", "err", err)
		}
	}

	p.log.Debug("", "src", peerAddr, "dst", p.listenAddr, downstreamJitterKey, jitter)

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient, peerAddr string) error {
	now := time.Now()

	if p.pl != nil {
		if err := p.pl.Record(p.listenAddr, peerAddr, "packetsent", now, nil); err != nil {
			p.log.Error("failed to record packet loss", "err", err)
		}
	}

	req := &pollpb.PollRequest{}
	req.SetTimestamp(timestamppb.New(now))
	resp, err := client.Poll(ctx, req)
	if err != nil {
		p.log.Warn("poll failed", "err", err)
		return err
	}

	rtt := time.Since(now)

	if p.pl != nil {
		if err := p.pl.Record(p.listenAddr, peerAddr, "packetrecv", now, nil); err != nil {
			p.log.Error("failed to record packet loss", "err", err)
		}
	}

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		return fmt.Errorf("no jitter in response")
	}
	jitter := jitterPb.AsDuration()

	if p.jitter != nil {
		if err := p.jitter.Record(p.listenAddr, peerAddr, upstreamJitterKey, now, &jitter); err != nil {
			p.log.Error("failed to record upstream jitter", "err", err)
		}
	}

	if p.rtt != nil {
		if err := p.rtt.Record(p.listenAddr, peerAddr, rttKey, now, &rtt); err != nil {
			p.log.Error("failed to record rtt", "err", err)
		}
	}

	p.log.Debug("", "src", p.listenAddr, "dst", peerAddr, rttKey, rtt)
	p.log.Debug("", "src", p.listenAddr, "dst", peerAddr, upstreamJitterKey, jitter)

	return nil
}
