package comms

// export for testing.
func (c *Client) GetPoller() DoPoller {
	return c.poller
}
