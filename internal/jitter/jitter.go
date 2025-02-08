// Package jitter implements https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1
package jitter

import (
	"time"
)

const gain int64 = 16

type Packet struct {
	S time.Time
	R time.Time
	J time.Duration
}

type PacketBuffer struct {
	I Packet // current packet
	J Packet // previous packet
}

func (b *PacketBuffer) Jitter() (time.Duration, bool) {
	i, j := b.I, b.J
	Ri, Rj := i.R, j.R
	Si, Sj := i.S, j.S
	Jj := j.J

	// Relative transit time difference:
	// D(i,j) = (Rj - Ri) - (Sj - Si) = (Rj - Sj) - (Ri - Si)
	// DiJ = (Rj - Sj) - (Ri - Si)
	Dij := (Rj.Sub(Ri) - (Sj.Sub(Si)))

	// Interarrival jitter:
	// J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
	// J(i) = Jj + (|Dij| - Jj)/16
	Ji := Jj + (Dij.Abs()-Jj)/time.Duration(gain)
	b.I.J = Ji

	return Ji, true
}

func (b *PacketBuffer) Push(e Packet) {
	// shift current packet to previous packet; set new packet as current packet
	b.J, b.I = b.I, e
}

type HostPacketBuffers map[string]PacketBuffer

func (b HostPacketBuffers) Jitter(hostID string) (time.Duration, bool) {
	buffer, ok := b[hostID]
	if !ok {
		return 0, false
	}

	return buffer.Jitter()
}

func (b HostPacketBuffers) Sample(hostID string, e Packet) {
	buffer := b[hostID] // get host packet buffer or new buffer if not exists
	buffer.Push(e)      // push new packet to buffer
	b[hostID] = buffer  // update host buffer
}
