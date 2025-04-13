package jitter

// export for testing.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.data)
}

// export for testing.
func (b *Buffer) PeerBufferLen(host string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.data[host])
}
