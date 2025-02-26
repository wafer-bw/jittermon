package latency

import "context"

// export for testing.
type ClientFunc = clientFunc

// export for testing.
func WithClientFunc(clientFunc ClientFunc) ClientOption {
	return func(c *Client) error {
		c.clientFunc = clientFunc
		return nil
	}
}

// export for testing.
func (c *Client) Sample(ctx context.Context) error {
	return c.sample(ctx)
}
