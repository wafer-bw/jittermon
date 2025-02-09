// Package jitter implements https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1
package jitter

import (
	"sync"
	"time"
)

const (
	minSamples int   = 2
	gain       int64 = 16
)

type Packet struct {
	S time.Time
	R time.Time
	j time.Duration
}

type HostPacketBuffers struct {
	mu   *sync.Mutex
	data map[string]packetBuffer
}

func NewHostPacketBuffers() HostPacketBuffers {
	return HostPacketBuffers{
		mu:   &sync.Mutex{},
		data: map[string]packetBuffer{},
	}
}

func (b HostPacketBuffers) Jitter(hostID string) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	buffer, ok := b.data[hostID]
	if !ok {
		return 0, false
	}

	return buffer.jitter()
}

func (b HostPacketBuffers) Sample(hostID string, e Packet) {
	b.mu.Lock()
	defer b.mu.Unlock()

	buffer := b.data[hostID] // get host packet buffer or new buffer if not exists
	buffer.push(e)           // push new packet to buffer
	b.data[hostID] = buffer  // update host buffer
}

type packetBuffer []Packet

func (b *packetBuffer) jitter() (time.Duration, bool) {
	if len(*b) < minSamples {
		return 0, false
	}

	i, j := (*b)[1], (*b)[0]
	Ri, Rj := i.R, j.R
	Si, Sj := i.S, j.S
	Jj := j.j

	// Relative transit time difference:
	// D(i,j) = (Rj - Ri) - (Sj - Si) = (Rj - Sj) - (Ri - Si)
	// DiJ = (Rj - Sj) - (Ri - Si)
	Dij := (Rj.Sub(Ri) - (Sj.Sub(Si)))

	// Interarrival jitter:
	// J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
	// J(i) = Jj + (|Dij| - Jj)/16
	Ji := Jj + (Dij.Abs()-Jj)/time.Duration(gain)
	(*b)[1].j = Ji

	return Ji, true
}

func (b *packetBuffer) push(e Packet) {
	// shift current packet to previous packet; set new packet as current packet
	*b = append(*b, e)
	if len(*b) > minSamples {
		*b = (*b)[1:]
	}
}
