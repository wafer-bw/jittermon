package traceroute

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type Hops []Hop

func (hs Hops) String() string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 2, 4, 2, ' ', 0)
	for _, h := range hs {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t\n", h.Hop, h.Name, h.Addr, h.RTT)
	}
	w.Flush()
	return buf.String()
}

type Hop struct {
	Addr string
	Name string
	Hop  int
	RTT  *time.Duration
}

func (h Hop) String() string {
	return fmt.Sprintf("%d %s %s %s", h.Hop, h.Addr, h.Name, h.RTT)
}

type Tracer struct {
	MaxHops int
	Timeout time.Duration
}

func (t Tracer) Trace(dst string) (Hops, error) {
	timeout := strconv.Itoa(int(t.Timeout.Seconds()))
	maxhops := strconv.Itoa(t.MaxHops)

	cmd := exec.Command("traceroute",
		"-w", timeout,
		"-m", maxhops,
		"-q", "1",
		dst,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running traceroute: %w", err)
	}
	output = bytes.TrimSpace(output)

	lines := strings.Split(string(output), "\n")
	hops := make([]Hop, 0, len(lines)-1)
	for i, line := range strings.Split(string(output), "\n") {
		if i == 0 {
			continue
		}

		line = strings.NewReplacer(
			"(", " ",
			")", " ",
			"  ", " ",
			" ms", "ms",
		).Replace(strings.TrimSpace(line))

		if strings.HasSuffix(line, "*") {
			hops = append(hops, Hop{Addr: "*", Name: "*", Hop: i})
			continue
		}

		var hop int
		var host, ip, rtt string
		if _, err := fmt.Sscanf(line, "%d %s %s %s", &hop, &host, &ip, &rtt); err != nil {
			return nil, fmt.Errorf("unable to parse traceroute line %d %q: %w", i, line, err)
		}

		rttDuration, err := time.ParseDuration(rtt)
		if err != nil {
			return nil, fmt.Errorf("unable to parse rtt %q: %w", rtt, err)
		}

		hops = append(hops, Hop{Addr: ip, Name: host, Hop: hop, RTT: &rttDuration})
	}

	return hops, nil
}
