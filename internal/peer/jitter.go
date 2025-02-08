package peer

import "time"

type PeerRequest struct {
	SentAt     time.Time
	ReceivedAt time.Time
}

type RequestBuffer struct {
	Sent          time.Time
	Received      time.Time
	ReceiveJitter time.Duration
	SendJitter    time.Duration

	// used to avoid reporting jitter until we have sampled twice (not a count
	// of total samples).
	s int
}

func (b RequestBuffer) Jitter() (time.Duration, bool) {
	if b.s != 2 {
		return 0, false
	}

	return (b.ReceiveJitter - b.SendJitter).Abs(), true
}

func (b *RequestBuffer) Sample(e PeerRequest) {
	b.ReceiveJitter = e.ReceivedAt.Sub(b.Received) - b.ReceiveJitter
	b.SendJitter = e.SentAt.Sub(b.Sent) - b.SendJitter
	b.Received = e.ReceivedAt
	b.Sent = e.SentAt
	if b.s < 2 {
		b.s++
	}
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
	buffer := b[peerID]
	buffer.Sample(e)
	b[peerID] = buffer
}
