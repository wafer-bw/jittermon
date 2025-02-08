package peer

import "time"

type PeerRequest struct {
	SentAt     time.Time
	ReceivedAt time.Time
}

type RequestBuffer []PeerRequest

func (b RequestBuffer) Jitter() (time.Duration, bool) {
	// TODO: there may be a more optimal way to do this math.

	if len(b) <= 2 {
		return 0, false
	}

	e1, e2, e3 := b[0], b[1], b[2]

	sendInterval1 := e2.SentAt.Sub(e1.SentAt)
	sendInterval2 := e3.SentAt.Sub(e2.SentAt)
	sendJitter := sendInterval2 - sendInterval1

	receiveInterval1 := e2.ReceivedAt.Sub(e1.ReceivedAt)
	receiveInterval2 := e3.ReceivedAt.Sub(e2.ReceivedAt)
	receiveJitter := receiveInterval2 - receiveInterval1

	return (receiveJitter - sendJitter).Abs(), true
}

func (b *RequestBuffer) Push(e PeerRequest) {
	*b = append(*b, e)
	if len(*b) > 3 {
		*b = (*b)[1:]
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

func (b PeerRequestBuffers) Add(peerID PeerID, e PeerRequest) {
	buffer := b[peerID]
	buffer.Push(e)
	b[peerID] = buffer
}
