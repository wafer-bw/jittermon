package latency

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wafer-bw/go-toolbox/graceful"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/pb/pollpb"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ graceful.Runner = (*Client)(nil)

const (
	clientName      string        = "latency client"
	DefaultInterval time.Duration = 1 * time.Second
)

var (
	defaultLogger         *slog.Logger = slog.New(slog.DiscardHandler)
	defaultClientGRPCOpts              = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
)

type ClientOption func(*Client) error

func ClientID(id string) ClientOption {
	return func(c *Client) error {
		c.id = id
		return nil
	}
}

func ClientInterval(interval time.Duration) ClientOption {
	return func(c *Client) error {
		c.interval = interval
		return nil
	}
}

func ClientRecorder(recorder rec.Recorder) ClientOption {
	return func(c *Client) error {
		c.recorder = recorder
		return nil
	}
}

func ClientLog(log *slog.Logger) ClientOption {
	return func(c *Client) error {
		c.log = log
		return nil
	}
}

// TODO: ClientEnv() ClientOption

func ClientGRPCDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *Client) error {
		c.grpcOpts = opts
		return nil
	}
}

type Client struct {
	id         string
	addr       string
	interval   time.Duration
	clientFunc clientFunc
	grpcOpts   []grpc.DialOption
	client     pollpb.PollServiceClient
	recorder   rec.Recorder
	log        *slog.Logger
	ticker     *time.Ticker
	stopCh     chan struct{}
	doneCh     chan struct{}
}

func NewClient(address string, options ...ClientOption) (*Client, error) {
	c := &Client{
		id:         littleid.New(),
		addr:       address,
		interval:   DefaultInterval,
		recorder:   rec.NoOp,
		clientFunc: defaultClientFunc,
		log:        defaultLogger,
		grpcOpts:   defaultClientGRPCOpts,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// TODO: verify this doesn't mutate the original logger
	c.log = c.log.With(
		"id", c.id,
		"name", clientName,
		"addr", c.addr,
	)

	return c, nil
}

func (c *Client) sample(ctx context.Context) error {
	start := time.Now()
	req := &pollpb.PollRequest{}
	req.SetId(c.id)
	req.SetTimestamp(timestamppb.New(start))

	packetLabels := rec.Labels{{K: "src", V: c.id}, {K: "dst", V: c.addr}}
	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: packetLabels})
	resp, err := c.client.Poll(ctx, req)
	if err != nil {
		c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: packetLabels})
		c.log.Error("poll failed", "err", err)
		return err
	}

	rtt := time.Since(start)

	dstID := resp.GetId()
	if dstID == "" {
		c.log.Error("no id in response")
		return fmt.Errorf("no id in response")
	}

	jitterPb := resp.GetJitter()
	if jitterPb == nil {
		c.log.Warn("no jitter in response")
		return fmt.Errorf("no jitter in response")
	}
	jit := jitterPb.AsDuration()

	latencyLabels := rec.Labels{{K: "src", V: c.id}, {K: "dst", V: dstID}}
	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeUpstreamJitter, Val: jit, Labels: latencyLabels})
	c.recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: latencyLabels})

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	c.log.Info("starting")
	defer close(c.doneCh)

	client, conn, err := c.clientFunc(c.addr, c.grpcOpts...)
	if err != nil {
		return err
	}
	defer conn.Close()
	c.client = client

	c.ticker = time.NewTicker(c.interval)
	defer c.ticker.Stop()

	for {
		select {
		case <-c.ticker.C:
			if err := c.sample(ctx); err != nil {
				c.log.Warn("sample failed", "err", err)
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.log.Debug("stopping")
	close(c.stopCh)

	select {
	case <-c.doneCh:
		// fallthrough
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s (%d) failed: %w", clientName, c.id, ctx.Err())
	}

	c.log.Debug("stopped")
	return nil
}

type clientFunc func(string, ...grpc.DialOption) (pollpb.PollServiceClient, *grpc.ClientConn, error)

func defaultClientFunc(t string, opts ...grpc.DialOption) (pollpb.PollServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(t, opts...)
	if err != nil {
		return nil, nil, err
	}

	client := pollpb.NewPollServiceClient(conn)

	return client, conn, nil
}
