package latency

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/wafer-bw/jittermon/internal/jitter"
	rec "github.com/wafer-bw/jittermon/internal/recorder"
)

const (
	SamplerName string = "latency"

	clientName      string = "udp_latency_client"
	replyBufferSize int    = 512
)

var packet = []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

type Recorder interface {
	rec.Recorder
}

type Client struct {
	ID             string
	Address        string
	Interval       time.Duration
	Recorder       Recorder
	RequestBuffers *jitter.Buffer
	Log            *slog.Logger
	StopCh         chan struct{}
	StoppedCh      chan struct{}
}

func (c Client) Poll(ctx context.Context) error {
	start := time.Now()
	labels := rec.Labels{{K: "src", V: c.ID}, {K: "dst", V: c.Address}}

	conn, err := net.Dial("udp", c.Address)
	if err != nil {
		c.Log.Error("failed to dial", "err", err)
		return err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(c.Interval))

	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeSentPackets, Val: struct{}{}, Labels: labels})
	if _, err := conn.Write(packet); err != nil {
		c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		c.Log.Error("failed to send packet", "err", err)
		return err
	}

	if _, err := conn.Read(make([]byte, replyBufferSize)); err != nil {
		c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeLostPackets, Val: struct{}{}, Labels: labels})
		c.Log.Error("failed to read packet", "err", err)
		return err
	}
	end := time.Now()
	rtt := end.Sub(start)

	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTT, Val: rtt, Labels: labels})

	jitter, ok := c.RequestBuffers.Interarrival(c.ID, start, end)
	if !ok {
		return nil
	}
	c.Recorder.Record(ctx, rec.Sample{Time: start, Type: rec.SampleTypeRTTJitter, Val: jitter, Labels: labels})

	return nil
}

func (c *Client) Start(ctx context.Context) error {
	c.Log = c.Log.With("id", c.ID, "name", clientName, "address", c.Address)
	c.Log.Info("starting")

	defer close(c.StoppedCh)

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.Poll(ctx); err != nil {
				c.Log.Warn("do poll failed", "err", err)
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-c.StopCh:
			return nil
		}
	}
}

func (c *Client) Stop(ctx context.Context) error {
	c.Log.Debug("stopping")

	close(c.StopCh)

	select {
	case <-c.StoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("graceful stop of %s[%s] failed: %w", clientName, c.ID, ctx.Err())
	}
}
