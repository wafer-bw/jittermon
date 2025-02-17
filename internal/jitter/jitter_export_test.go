package jitter

// export for testing.
func (b HostPacketBuffers) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.data)
}

// export for testing.
func (b HostPacketBuffers) HostBufferLen(host string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.data[host])
}
