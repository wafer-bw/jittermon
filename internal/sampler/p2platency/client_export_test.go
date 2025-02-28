package p2platency

import (
	"context"
	"log/slog"
	"time"

	"github.com/wafer-bw/jittermon/internal/recorder"
)

// export for testing.
func (c *Client) Sample(ctx context.Context) error {
	return c.sample(ctx)
}

// export for testing.
func (c *Client) GetID() string {
	return c.id
}

// export for testing.
func (c *Client) GetLogger() *slog.Logger {
	return c.log
}

// export for testing.
func (c *Client) GetRecorder() recorder.Recorder {
	return c.recorder
}

// export for testing.
func (c *Client) GetInterval() time.Duration {
	return c.interval
}
