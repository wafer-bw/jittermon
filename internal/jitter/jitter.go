// Package jitter implements interarrival jitter as per RFC3550 Section 6.4.1
package jitter

import (
	"sync"
	"time"
)

const (
	minSamples int   = 2
	gain       int64 = 16 // https://datatracker.ietf.org/doc/html/rfc3550#ref-22
)

type packet struct {
	sentAt             time.Time
	receivedAt         time.Time
	interarrivalJitter time.Duration
}

// Buffer allows sampling jitter for multiple peers concurrently.
//
// This type embeds a non-pointer mutex so it can be used without a constructor,
// so when you use this type make sure to use it as a pointer like:
//
//	buffer := &jitter.Buffer{}
type Buffer struct {
	mu   sync.RWMutex
	data map[string][]packet
}

// Interarrival jitter for a peer calculated using the sent and received time of
// an incoming packet calculated as per RFC3550 Section 6.4.1:
// https://datatracker.ietf.org/doc/html/rfc3550#section-6.4.1
//
// This method is thread-safe and can be called concurrently from multiple
// goroutines.
func (b *Buffer) Interarrival(pid string, sentAt, receivedAt time.Time) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	p := packet{sentAt: sentAt, receivedAt: receivedAt}

	// lazy initialization of map so we don't need a constructor function.
	if b.data == nil {
		b.data = map[string][]packet{}
	}

	packets := b.data[pid]         // get peer packets
	packets = append(packets, p)   // add new packet
	if len(packets) > minSamples { // remove oldest packet
		packets = packets[1:]
	}

	// make sure we update the map with the new packet slice, this must happen
	// before any return from this function so we defer it.
	defer func() { b.data[pid] = packets }()

	if len(packets) < minSamples { // not enough samples yet
		return 0, false
	}

	// name vars as per RFC3550 so it's easy to read in a mathematical context.
	i := packets[1] // current packet
	j := packets[0] // previous packet (i-1)
	Jj := j.interarrivalJitter
	Ri, Rj := i.receivedAt, j.receivedAt
	Si, Sj := i.sentAt, j.sentAt

	// relative transit time difference:
	// D(i,j) = (Rj - Ri) - (Sj - Si) = (Rj - Sj) - (Ri - Si)
	// Dij = (Rj - Sj) - (Ri - Si)
	Dij := (Rj.Sub(Ri) - (Sj.Sub(Si)))

	// interarrival jitter:
	// J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
	// J(i) = Jj + (|Dij| - Jj)/16
	Ji := Jj + (Dij.Abs()-Jj)/time.Duration(gain)

	packets[1].interarrivalJitter = Ji

	return Ji, true
}
