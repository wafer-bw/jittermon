// TODO: switch to pure go implementation of traceroute.
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
	w := tabwriter.NewWriter(&buf, 2, 4, 2, ' ', 0) //nolint:mnd // not reused.
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

type TraceOptions struct {
	MaxHops int
	Timeout time.Duration
}

func Trace(dst string, opts TraceOptions) (Hops, error) {
	args := []string{"-q", "1"}
	if opts.MaxHops > 0 {
		args = append(args, "-m", strconv.Itoa(opts.MaxHops))
	}
	if opts.Timeout > 0 {
		args = append(args, "-w", strconv.Itoa(int(opts.Timeout.Seconds())))
	}
	args = append(args, dst)

	output, err := exec.Command("traceroute", args...).CombinedOutput()
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

		hop, host, ip, rtt := 0, "", "", ""
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
