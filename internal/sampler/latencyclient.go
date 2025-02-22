package sampler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var none struct{} = struct{}{}

type LatencyClient struct {
	id       string
	addr     string
	interval time.Duration
	ticker   *time.Ticker
	recorder rec.Recorder
	client   pollpb.PollServiceClient
	log      *slog.Logger
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type LatencyClientOptions struct {
	ID       string
	Address  string
	Interval time.Duration
	Recorder rec.Recorder
	Log      *slog.Logger
}

func NewLatencyClient(opts LatencyClientOptions) (*LatencyClient, error) {
	if opts.Interval == 0 {
		return nil, fmt.Errorf("sampler.NewLatencyClient: interval is required")
	}

	lc := &LatencyClient{
		id:       opts.ID,
		addr:     opts.Address,
		interval: opts.Interval,
		recorder: opts.Recorder,
		log:      slog.New(slog.DiscardHandler),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	if opts.Log != nil {
		lc.log = opts.Log
	}

	return lc, nil
}

func (lc *LatencyClient) Start(ctx context.Context) error {
	defer close(lc.doneCh)
	lc.log.Info("starting client", "addr", lc.addr)

	conn, err := grpc.NewClient(lc.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	if lc.client == nil {
		lc.client = pollpb.NewPollServiceClient(conn)
	}

	lc.ticker = time.NewTicker(lc.interval)
	defer lc.ticker.Stop()

	for {
		select {
		case <-lc.ticker.C:
			if err := lc.sample(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-lc.stopCh:
			return nil
		}
	}
}

func (lc *LatencyClient) Stop(ctx context.Context) error {
	close(lc.stopCh)

	select {
	case <-lc.doneCh:
		// fallthrough
	case <-ctx.Done():
		return fmt.Errorf("graceful stop failed: %w", ctx.Err())
	}

	return nil
}

func (lc *LatencyClient) sample(ctx context.Context) error {
	start := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(lc.id)
	req.SetTimestamp(timestamppb.New(start))

	packetLabels := rec.Labels{{K: "src", V: lc.id}, {K: "dst", V: lc.addr}}
	lc.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: none, Labels: packetLabels})
	resp, err := lc.client.Poll(ctx, req)
	if err != nil {
		lc.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: none, Labels: packetLabels})
		lc.log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(start)

	dstID := resp.GetId()
	if dstID == "" {
		lc.log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		lc.log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()

	latencyLabels := rec.Labels{{K: "src", V: lc.id}, {K: "dst", V: dstID}}
	lc.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeUpstreamJitter, Val: jit, Labels: latencyLabels})
	lc.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: latencyLabels})

	return nil
}
