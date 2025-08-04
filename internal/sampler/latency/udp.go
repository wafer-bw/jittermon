package latency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	"github.com/wafer-bw/jittermon/internal/littleid"
	"github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	UDPName string = "udp_latency_client"

	defaultInterval time.Duration = 1 * time.Second
	defaultTimeout  time.Duration = defaultInterval * time.Duration(2)

	replyBufferSize int = 512
)

var (
	defaultLog = slog.New(slog.DiscardHandler)
	packet     = []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

type UDPOption func(*UDPClient) error

func WithUDPID(id string) UDPOption {
	return func(c *UDPClient) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		c.id = id
		return nil
	}
}

func WithUDPInterval(interval time.Duration) UDPOption {
	return func(c *UDPClient) error {
		if interval <= 0 {
			return nil
		}
		c.interval = interval
		return nil
	}
}

func WithUDPTimeout(timeout time.Duration) UDPOption {
	return func(c *UDPClient) error {
		if timeout <= 0 {
			return nil
		}
		c.timeout = timeout
		return nil
	}
}

func WithUDPLog(log *slog.Logger) UDPOption {
	return func(c *UDPClient) error {
		if log == nil {
			return nil
		}
		c.log = log
		return nil
	}
}

type UDPClient struct {
	id       string
	address  string
	interval time.Duration
	timeout  time.Duration
	recorder Recorder
	log      *slog.Logger
	jitter   *jitter.Buffer
}

func NewUDP(address string, recorder Recorder, options ...UDPOption) (*UDPClient, error) {
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	} else if recorder == nil {
		return nil, fmt.Errorf("recorder cannot be nil")
	}

	c := &UDPClient{
		address:  address,
		recorder: recorder,
		id:       littleid.New(),
		jitter:   &jitter.Buffer{},
		interval: defaultInterval,
		timeout:  defaultTimeout,
		log:      defaultLog,
	}

	for _, option := range options {
		if err := option(c); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	c.log = c.log.With("name", UDPName, "id", c.id, "address", c.address)

	return c, nil
}

func (c *UDPClient) Poll(ctx context.Context) error {
	labels := recorder.Labels{{K: "src", V: c.id}, {K: "dst", V: c.address}}

	start := time.Now()
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	if err := c.poll(ctx); err != nil {
		c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		return err
	}
	end := time.Now()

	rtt := end.Sub(start)
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeRTT, Val: rtt, Labels: labels})

	jitter, ok := c.jitter.Interarrival(c.id, start, end)
	if !ok {
		return fmt.Errorf("no jitter in response")
	}
	c.recorder.Record(ctx, recorder.Sample{Time: start, Type: recorder.SampleTypeRTTJitter, Val: jitter, Labels: labels})

	return nil
}

func (c *UDPClient) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.InfoContext(ctx, "starting", "interval", c.interval)

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				c.log.WarnContext(ctx, "poll failed", "err", err)
				continue
			}
		case <-ctx.Done():
			c.log.WarnContext(ctx, "context done, stopping", "err", ctx.Err())
			return ctx.Err()
		}
	}
}

func (c *UDPClient) poll(_ context.Context) error {
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
