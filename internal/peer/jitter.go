package peer

import "time"

type PeerRequest struct {
	sentAt     time.Time
	receivedAt time.Time
}

type RequestBuffer []PeerRequest

func (b RequestBuffer) Jitter() (time.Duration, bool) {
	if len(b) <= 2 {
		return 0, false
	}

	e1, e2, e3 := b[0], b[1], b[2]

	sendInterval1 := e2.sentAt.Sub(e1.sentAt)
	sendInterval2 := e3.sentAt.Sub(e2.sentAt)
	sendJitter := sendInterval2 - sendInterval1

	receiveInterval1 := e2.receivedAt.Sub(e1.receivedAt)
	receiveInterval2 := e3.receivedAt.Sub(e2.receivedAt)
	receiveJitter := receiveInterval2 - receiveInterval1

	jitter := (receiveJitter - sendJitter).Abs()

	return jitter, true
}

func (b *RequestBuffer) Push(e PeerRequest) {
	*b = append(*b, e)
	if len(*b) > 3 {
		*b = (*b)[1:]
	}
}

type PeerRequestBuffers map[string]RequestBuffer

func (b PeerRequestBuffers) Jitter(peerID string) (time.Duration, bool) {
	buffer, ok := b[peerID]
	if !ok {
		return 0, false
	}

	return buffer.Jitter()
}

func (b PeerRequestBuffers) Add(peerID string, e PeerRequest) {
	buffer, ok := b[peerID]
	if !ok {
		buffer = RequestBuffer{}
	}

	buffer.Push(e)
	b[peerID] = buffer
}
