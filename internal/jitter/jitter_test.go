package jitter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/jitter"
)

func TestHostPacketBuffers_Jitter(t *testing.T) {
	t.Parallel()

	t.Run("populates buffer & measures zero jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now,
			R: now.Add(50 * time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2 * time.Second),
			R: now.Add(2*time.Second + 50*time.Millisecond),
		})

		require.Equal(t, 1, b.Len())
		require.Equal(t, 2, b.HostBufferLen(pid))

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, time.Duration(0), jitter)
	})

	t.Run("handles sampling via multiple threads safely", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		b := jitter.NewHostPacketBuffers()

		for range 1000 {
			go b.Sample(pid, jitter.Packet{
				S: time.Now(),
				R: time.Now().Add(50 * time.Millisecond),
			})
		}
	})

	t.Run("measures delayed jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2 * time.Second),
			R: now.Add(2*time.Second + 60*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 625*time.Microsecond, jitter)
	})

	t.Run("measures hastened jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2 * time.Second),
			R: now.Add(2*time.Second + 40*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 625*time.Microsecond, jitter)
	})

	t.Run("negates delayed send jitter from delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2*time.Second + 5*time.Millisecond),
			R: now.Add(2*time.Second + 60*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 312500*time.Nanosecond, jitter)
	})

	t.Run("negates hastened send jitter from hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2*time.Second - 5*time.Millisecond),
			R: now.Add(2*time.Second + 40*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 312500*time.Nanosecond, jitter)
	})

	t.Run("combines hastened send jitter with delayed receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2*time.Second - 5*time.Millisecond),
			R: now.Add(2*time.Second + 60*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 937500*time.Nanosecond, jitter)
	})

	t.Run("combines delayed send jitter with hastened receive jitter", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})
		b.Sample(pid, jitter.Packet{
			S: now.Add(2*time.Second + 5*time.Millisecond),
			R: now.Add(2*time.Second + 40*time.Millisecond),
		})

		jitter, ok := b.Jitter(pid)
		require.True(t, ok)
		require.Equal(t, 937500*time.Nanosecond, jitter)
	})

	t.Run("does not report jitter until two packets have been sampled", func(t *testing.T) {
		t.Parallel()

		pid := "1"
		now := time.Now()
		b := jitter.NewHostPacketBuffers()

		_, ok := b.Jitter(pid)
		require.False(t, ok)

		b.Sample(pid, jitter.Packet{
			S: now,
			R: now.Add(50 * time.Millisecond),
		})

		_, ok = b.Jitter(pid)
		require.False(t, ok)

		b.Sample(pid, jitter.Packet{
			S: now.Add(1 * time.Second),
			R: now.Add(1*time.Second + 50*time.Millisecond),
		})

		_, ok = b.Jitter(pid)
		require.True(t, ok)
	})
}
