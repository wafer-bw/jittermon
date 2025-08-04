package grpcp2platency

// export for testing.
func WithClientConn(conn ClientPoller) ClientOption {
	return func(c *Client) error {
		c.conn = conn
		return nil
	}
}
