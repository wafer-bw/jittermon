package exectraceroute_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/sampler/exectraceroute"
)

func TestHop_String(t *testing.T) {
	t.Parallel()

	hop := exectraceroute.Hop{Addr: "192.168.1.1", Name: "192.168.1.1", Hop: 1, RTT: ptr(828 * time.Microsecond)}
	expected := "1 192.168.1.1 192.168.1.1 828µs"
	require.Equal(t, expected, hop.String())
}

func TestHops_String(t *testing.T) {
	t.Parallel()

	hops := exectraceroute.Hops{
		{Addr: "192.168.1.1", Name: "192.168.1.1", Hop: 1, RTT: ptr(828 * time.Microsecond)},
		{Addr: "1.1.1.1", Name: "something.example.com", Hop: 2, RTT: ptr(14177 * time.Microsecond)},
	}
	expected := "1  192.168.1.1            192.168.1.1  828µs     \n2  something.example.com  1.1.1.1      14.177ms  \n"
	require.Equal(t, expected, hops.String())
}
