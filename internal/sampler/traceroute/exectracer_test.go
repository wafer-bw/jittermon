package traceroute_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/sampler/traceroute"
)

func TestExecTracer(t *testing.T) {
	t.Parallel()

	t.Run("successful trace", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		expectedHops := traceroute.Hops{
			{Addr: "192.168.1.1", Name: "192.168.1.1", Hop: 1, RTT: ptr(time.Microsecond * 828)},
			{Addr: "1.1.1.1", Name: "something.example.com", Hop: 2, RTT: ptr(time.Microsecond * 14177)},
			{Addr: "*", Name: "*", Hop: 3, RTT: nil},
			{Addr: "3.3.3.3", Name: "somethingelse.example.com", Hop: 4, RTT: ptr(time.Microsecond * 12981)},
			{Addr: "8.8.8.8", Name: "dns.google", Hop: 5, RTT: ptr(time.Microsecond * 12849)},
		}
		execfn := func(name string, args ...string) (string, error) {
			require.Equal(t, "-q", args[0])
			require.Equal(t, "1", args[1])
			require.Equal(t, "-m", args[2])
			require.Equal(t, "10", args[3])
			require.Equal(t, "-w", args[4])

			return `
				traceroute to 8.8.8.8 (8.8.8.8), 64 hops max, 40 byte packets
				1  192.168.1.1 (192.168.1.1)  0.828 ms
				2  something.example.com (1.1.1.1)  14.177 ms
				3  *
				4  somethingelse.example.com (3.3.3.3)  12.981 ms
				5  dns.google (8.8.8.8)  12.849 ms
			`, nil
		}

		tr := &traceroute.ExecTracer{ExecFn: execfn, MaxHops: 10, Timeout: 1 * time.Second}
		hops, err := tr.Trace(ctx, "8.8.8.8")
		require.NoError(t, err)
		require.Equal(t, expectedHops, hops)
	})

	t.Run("returns error returned by exec function", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		execfn := func(name string, args ...string) (string, error) {
			return "", errors.New("error")
		}

		tr := &traceroute.ExecTracer{ExecFn: execfn}
		hops, err := tr.Trace(ctx, "8.8.8.8")
		require.Error(t, err)
		require.Empty(t, hops)
	})
}

func ptr[T any](v T) *T {
	return &v
}
