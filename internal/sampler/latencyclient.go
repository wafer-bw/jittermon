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
	ticker   *time.Ticker
	recorder rec.Recorder
	log      *slog.Logger
}

type LatencyClientOptions struct {
	ID       string
	Address  string
	Interval time.Duration
	Recorder rec.Recorder
	Log      *slog.Logger
}

func NewLatencyClient(opts LatencyClientOptions) (LatencyClient, error) {
	if opts.Interval == 0 {
		return LatencyClient{}, fmt.Errorf("sampler.NewLatencyClient: interval is required")
	}

	lc := LatencyClient{
		id:       opts.ID,
		addr:     opts.Address,
		recorder: opts.Recorder,
		ticker:   time.NewTicker(opts.Interval), // TODO: this likely needs to be made in Start.
		log:      slog.New(slog.DiscardHandler),
	}

	if opts.Log != nil {
		lc.log = opts.Log
	}

	return lc, nil
}

func (lc LatencyClient) Start(ctx context.Context) error {
	lc.log.Info("starting client", "addr", lc.addr)

	conn, err := grpc.NewClient(lc.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	defer lc.ticker.Stop()

	client := pollpb.NewPollServiceClient(conn)

	for {
		select {
		case <-lc.ticker.C:
			if err := lc.sample(ctx, client); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (lc LatencyClient) Stop(ctx context.Context) error {
	lc.ticker.Stop()
	return nil
}

func (lc LatencyClient) sample(ctx context.Context, client pollpb.PollServiceClient) error {
	start := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(lc.id)
	req.SetTimestamp(timestamppb.New(start))

	packetLabels := rec.Labels{{K: "src", V: lc.id}, {K: "dst", V: lc.addr}}
	lc.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: none, Labels: packetLabels})
	resp, err := client.Poll(ctx, req)
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
