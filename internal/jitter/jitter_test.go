package jitter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/jitter"
)

func TestBuffer_Interarrival(t *testing.T) {
	t.Parallel()

	t.Run("populates buffer & measures zero jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now, now.Add(50*time.Millisecond))
		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		j, ok := b.Interarrival(pid, now.Add(2*time.Second), now.Add(2*time.Second+50*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, time.Duration(0), j)
		require.Equal(t, 1, b.Len())
		require.Equal(t, 2, b.PeerBufferLen(pid))
	})

	t.Run("handles sampling via multiple threads safely", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		b := jitter.Buffer{}

		require.NotPanics(t, func() {
			for range 1000 {
				go b.Interarrival(pid, time.Now(), time.Now().Add(50*time.Millisecond))
			}
		})
	})

	t.Run("measures delayed jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second), now.Add(2*time.Second+60*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 625*time.Microsecond, jitter)
	})

	t.Run("measures hastened jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second), now.Add(2*time.Second+40*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 625*time.Microsecond, jitter)
	})

	t.Run("negates delayed send jitter from delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second+5*time.Millisecond), now.Add(2*time.Second+60*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 312500*time.Nanosecond, jitter)
	})

	t.Run("negates hastened send jitter from hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second-5*time.Millisecond), now.Add(2*time.Second+40*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 312500*time.Nanosecond, jitter)
	})

	t.Run("combines hastened send jitter with delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second-5*time.Millisecond), now.Add(2*time.Second+60*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 937500*time.Nanosecond, jitter)
	})

	t.Run("combines delayed send jitter with hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, _ = b.Interarrival(pid, now.Add(1*time.Second), now.Add(1*time.Second+50*time.Millisecond))
		jitter, ok := b.Interarrival(pid, now.Add(2*time.Second+5*time.Millisecond), now.Add(2*time.Second+40*time.Millisecond))
		require.True(t, ok)
		require.Equal(t, 937500*time.Nanosecond, jitter)
	})

	t.Run("does not report jitter until two packets have been sampled", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.Buffer{}

		_, ok := b.Interarrival(pid, now, now.Add(50*time.Millisecond))
		require.False(t, ok)

		_, ok = b.Interarrival(pid, now, now.Add(50*time.Millisecond))
		require.True(t, ok)
	})
}
