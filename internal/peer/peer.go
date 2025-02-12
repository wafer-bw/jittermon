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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ pollpb.PollServiceServer = (*Peer)(nil)
var _ comms.DoPoller = (*Peer)(nil)

type Recorder interface {
	Record(context.Context, MetricSample)
}

type RecorderFunc func(context.Context, MetricSample)

func (f RecorderFunc) Record(ctx context.Context, s MetricSample) {
	f(ctx, s)
}

type Recorders []func(Recorder) Recorder

func (rs Recorders) Chain() Recorder {
	terminal := RecorderFunc(func(ctx context.Context, s MetricSample) { return })
	if len(rs) == 0 {
		return terminal
	}

	r := rs[len(rs)-1](terminal)
	for i := len(rs) - 1; i >= 0; i-- {
		r = rs[i](r)
	}

	return r
}

type MetricType string

const (
	MetricTypeDownstreamJitter MetricType = "downstream_jitter"
	MetricTypeUpstreamJitter   MetricType = "upstream_jitter"
	MetricTypeSentPackets      MetricType = "sent_packets"
	MetricTypeLostPackets      MetricType = "lost_packets"
	MetricTypeRTT              MetricType = "rtt"
)

type MetricSample struct {
	Time time.Time
	Type MetricType
	Src  string
	Dst  string
	Val  any
}

type Option func(*Peer) error

func WithLogger(log *slog.Logger) Option {
	return func(p *Peer) error {
		p.log = log
		return nil
	}
}

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

func WithRecorders(recorders ...func(Recorder) Recorder) Option {
	return func(p *Peer) error {
		p.r = Recorders(recorders).Chain()
		return nil
	}
}

type Peer struct {
	id             string
	log            *slog.Logger
	r              Recorder
	requestBuffers jitter.HostPacketBuffers

	pollpb.UnimplementedPollServiceServer
}

func NewPeer(opts ...Option) (*Peer, error) {
	p := &Peer{
		id:             strings.Split(uuid.New().String(), "-")[1],
		log:            slog.New(slog.DiscardHandler),
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

	p.r.Record(ctx, MetricSample{Time: now, Type: MetricTypeDownstreamJitter, Src: srcID, Dst: p.id, Val: jitter})
	resp.SetJitter(durationpb.New(jitter))

	return resp, nil
}

// DoPoll sends outgoing poll requests.
func (p *Peer) DoPoll(ctx context.Context, client pollpb.PollServiceClient, dstAddr string) error {
	now := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(p.id)
	req.SetTimestamp(timestamppb.New(now))

	p.r.Record(ctx, MetricSample{Time: now, Type: MetricTypeSentPackets, Src: p.id, Dst: dstAddr, Val: struct{}{}})
	resp, err := client.Poll(ctx, req)
	if err != nil {
		p.r.Record(ctx, MetricSample{Time: now, Type: MetricTypeLostPackets, Src: p.id, Dst: dstAddr, Val: struct{}{}})
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

	p.r.Record(ctx, MetricSample{Time: now, Type: MetricTypeUpstreamJitter, Src: p.id, Dst: dstID, Val: jitter})
	p.r.Record(ctx, MetricSample{Time: now, Type: MetricTypeRTT, Src: p.id, Dst: dstID, Val: rtt})

	return nil
}
