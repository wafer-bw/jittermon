package p2platency

// export for testing.
func WithGRPCClientConn(conn GRPCClientPoller) GRPCClientOption {
	return func(c *GRPCClient) error {
		c.conn = conn
		return nil
	}
}
