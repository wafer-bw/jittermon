package peer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wafer-bw/jittermon/internal/comms"
	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	"github.com/wafer-bw/jittermon/internal/recorder"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ pollpb.PollServiceServer = (*Peer)(nil)
var _ comms.DoPoller = (*Peer)(nil)

// Option is a function that configures a peer, used via [NewPeer].
type Option func(*Peer) error

// WithID sets the id of the peer. If no id is provided a 4 character random one
// will be generated.
func WithID(id string) Option {
	return func(p *Peer) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		p.id = id
		return nil
	}
}

// WithLogger sets the [slog.Logger] the peer will use. If no logger is provided
// a handler using [slog.DiscardHandler] will be used.
func WithLogger(log *slog.Logger) Option {
	return func(p *Peer) error {
		p.log = log
		return nil
	}
}

// WithRecorders sets the recorders the peer will use. They will be chained &
// execute in the order they are provided.
func WithRecorders(recorders ...recorder.ChainLink) Option {
	return func(p *Peer) error {
		p.r = rec.Chain(recorders...)
		return nil
	}
}

// Peer is capable of handling & sending incoming & outgoing requests
// respectively. Always construct a new peer using [NewPeer].
type Peer struct {
	id             string
	log            *slog.Logger
	r              rec.Recorder
	requestBuffers jitter.HostPacketBuffers

	pollpb.UnimplementedPollServiceServer
}

func NewPeer(opts ...Option) (*Peer, error) {
	p := &Peer{
		id:             strings.Split(uuid.New().String(), "-")[1],
		log:            slog.New(slog.DiscardHandler),
		r:              recorder.RecorderFunc(func(_ context.Context, _ recorder.Sample) {}),
		requestBuffers: jitter.NewHostPacketBuffers(),
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	return p, nil
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

	p.r.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeDownstreamJitter, Src: srcID, Dst: p.id, Val: jitter})
	resp.SetJitter(durationpb.New(jitter))

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient, dstAddr string) error {
	now := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(p.id)
	req.SetTimestamp(timestamppb.New(now))

	p.r.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeSentPackets, Src: p.id, Dst: dstAddr, Val: struct{}{}})
	resp, err := client.Poll(ctx, req)
	if err != nil {
		p.r.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeLostPackets, Src: p.id, Dst: dstAddr, Val: struct{}{}})
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
		p.log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jitter := jitterPb.AsDuration()

	p.r.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeUpstreamJitter, Src: p.id, Dst: dstID, Val: jitter})
	p.r.Record(ctx, rec.Sample{Time: now, Type: rec.SampleTypeRTT, Src: p.id, Dst: dstID, Val: rtt})

	return nil
}
