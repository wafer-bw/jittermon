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

const (
	name             string        = "udp ptx client"
	defaultInterval  time.Duration = 1 * time.Second
	defaultTimeout   time.Duration = defaultInterval * time.Duration(2)
	defaultPollGrace int           = 1
	replyBufferSize  int           = 512
)

var (
	defaultLog = slog.New(slog.DiscardHandler)
	packet     = []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

type Option func(*Client) error

func WithInterval(interval time.Duration) Option {
	return func(c *Client) error {
		if interval <= 0 {
			return nil
		}
		c.interval = interval
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout <= 0 {
			return nil
		}
		c.timeout = timeout
		return nil
	}
}

func WithLog(log *slog.Logger) Option {
	return func(c *Client) error {
		if log == nil {
			return nil
		}
		c.log = log
		return nil
	}
}

type Client struct {
	id        string
	address   string
	interval  time.Duration
	timeout   time.Duration
	log       *slog.Logger
	jitter    *jitter.Buffer
	pollGrace int
}

func New(id string, address string, options ...Option) (*Client, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	} else if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	c := &Client{
		id:        id,
		address:   address,
		jitter:    &jitter.Buffer{},
		interval:  defaultInterval,
		timeout:   defaultTimeout,
		log:       defaultLog,
		pollGrace: defaultPollGrace,
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return c, nil
}

func (c *Client) Poll(ctx context.Context) error {
	attributes := metric.WithAttributes(
		attribute.String(otel.SourceLabelName, c.id),
		attribute.String(otel.DestinationLabelName, c.address),
	)

	start := time.Now()

	otel.SentPacketsCounter.Add(ctx, 1, attributes)
	c.log.DebugContext(ctx, otel.SentPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)
	if err := c.poll(ctx); err != nil {
		otel.LostPacketsCounter.Add(ctx, 1, attributes)
		c.log.DebugContext(ctx, otel.LostPacketsMetricName, "value", strconv.Itoa(1), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)
		return err
	}
	end := time.Now()

	rtt := end.Sub(start)
	otel.PingHistogram.Record(ctx, rtt.Seconds(), attributes)
	c.log.DebugContext(ctx, otel.PingMetricName, "value", strconv.FormatFloat(rtt.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)

	jitter, ok := c.jitter.Interarrival(c.id, start, end)
	if !ok {
		return fmt.Errorf("no jitter in response")
	}
	otel.JitterHistogram.Record(ctx, jitter.Seconds(), attributes)
	c.log.DebugContext(ctx, otel.JitterMetricName, "value", strconv.FormatFloat(jitter.Seconds(), 'f', 6, 64), otel.SourceLabelName, c.id, otel.DestinationLabelName, c.address)

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.InfoContext(ctx, "starting", "name", name, "address", c.address, "interval", c.interval)

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				if c.pollGrace > 0 {
					c.pollGrace--
				} else {
					c.log.ErrorContext(ctx, "poll failed", "name", name, "err", err)
				}
				continue
			}
		case <-ctx.Done():
			c.log.WarnContext(ctx, "context done, stopping", "name", name, "err", ctx.Err())
			return ctx.Err()
		}
	}
}

// TODO: implement.
func (c *Client) Stop(ctx context.Context) error {
	return nil
}

func (c *Client) poll(_ context.Context) error {
	conn, err := net.Dial("udp", c.address)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
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
