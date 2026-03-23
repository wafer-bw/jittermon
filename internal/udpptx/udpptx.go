package udpptx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Int64Counter interface {
	Add(ctx context.Context, incr int64, options ...metric.AddOption)
}

type Float64Histogram interface {
	Record(ctx context.Context, incr float64, options ...metric.RecordOption)
}

const (
	name              string        = "udp ptx client"
	defaultInterval   time.Duration = 1 * time.Second
	startingPollGrace int           = 1
	replyBufferSize   int           = 512
)

var (
	defaultLog = slog.New(slog.DiscardHandler)
	packet     = []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

type Client struct {
	ID                 string
	Address            string
	SentPacketsCounter Int64Counter
	LostPacketsCounter Int64Counter
	PingHistogram      Float64Histogram
	JitterHistogram    Float64Histogram

	Interval time.Duration
	Timeout  time.Duration
	Log      *slog.Logger

	pollGrace int
	jitter    *jitter.Buffer
}

func (c *Client) Poll(ctx context.Context) error {
	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, c.ID),
		attribute.String(otel.DestinationLabelName, c.Address),
	)
	start := time.Now()
	if c.jitter == nil {
		c.jitter = &jitter.Buffer{}
	}
	if c.Log == nil {
		c.Log = defaultLog
	}

	c.SentPacketsCounter.Add(ctx, 1, attributes)
	c.Log.DebugContext(ctx, otel.SentPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)
	if err := c.poll(ctx); err != nil {
		fmt.Println(err)
		c.LostPacketsCounter.Add(ctx, 1, attributes)
		c.Log.DebugContext(ctx, otel.LostPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)
		return err
	}
	end := time.Now()

	rtt := end.Sub(start)
	c.PingHistogram.Record(ctx, rtt.Seconds(), attributes)
	c.Log.DebugContext(ctx, otel.PingMetricName, "value", strconv.FormatFloat(rtt.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)

	jitter, ok := c.jitter.Interarrival(c.ID, start, end)
	if !ok {
		return fmt.Errorf("no jitter in response")
	}
	c.JitterHistogram.Record(ctx, jitter.Seconds(), attributes)
	c.Log.DebugContext(ctx, otel.JitterMetricName, "value", strconv.FormatFloat(jitter.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.ID, otel.DestinationLabelName, c.Address)

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	if c.ID == "" {
		return fmt.Errorf("%s client start: id cannot be empty", name)
	} else if c.Address == "" {
		return fmt.Errorf("%s client start: address cannot be empty", name)
	} else if c.Interval <= 0 {
		c.Interval = defaultInterval
	}

	if c.Timeout <= 0 {
		c.Timeout = c.Interval
	}
	if c.Log == nil {
		c.Log = defaultLog
	}
	c.pollGrace = startingPollGrace
	c.jitter = &jitter.Buffer{}

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	c.Log.InfoContext(ctx, "starting", "name", name, "address", c.Address, "interval", c.Interval)

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				if c.pollGrace > 0 {
					c.pollGrace--
				} else {
					c.Log.ErrorContext(ctx, "poll failed", "name", name, "err", err)
				}
				continue
			}
		case <-ctx.Done():
			c.Log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
			return ctx.Err()
		}
	}
}

func (c *Client) poll(_ context.Context) error {
	conn, err := net.Dial("udp", c.Address)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(c.Timeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	if _, err := conn.Read(make([]byte, replyBufferSize)); err != nil {
		return fmt.Errorf("read: %w", err)
	}

	return nil
}
