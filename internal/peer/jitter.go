package peer

import (
	"time"
)

// https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1

const gain int64 = 16

type PeerRequest struct {
	S time.Time
	R time.Time
	J time.Duration
}

type RequestBuffer struct {
	I PeerRequest // current packet
	J PeerRequest // previous packet
}

func (b *RequestBuffer) Jitter() (time.Duration, bool) {
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

func (b *RequestBuffer) Push(e PeerRequest) {
	// shift current packet to previous packet; set new packet as current packet
	b.J, b.I = b.I, e
}

type PeerRequestBuffers map[PeerID]RequestBuffer

func (b PeerRequestBuffers) Jitter(peerID PeerID) (time.Duration, bool) {
	buffer, ok := b[peerID]
	if !ok {
		return 0, false
	}

	return buffer.Jitter()
}

func (b PeerRequestBuffers) Sample(peerID PeerID, e PeerRequest) {
	buffer := b[peerID] // get peer buffer or new buffer if not exists
	buffer.Push(e)      // push new request to buffer
	b[peerID] = buffer  // update peer buffer
}
