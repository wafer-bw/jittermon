package traceroute

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var _ Tracer = (*execTracer)(nil)

var defaultExecFn = func(name string, arg ...string) (string, error) {
	b, err := exec.Command(name, arg...).CombinedOutput()
	return string(b), err
}

type execTracer struct {
	MaxHops int
	Timeout time.Duration
	ExecFn  func(name string, arg ...string) (string, error)
}

func (t *execTracer) Trace(_ context.Context, dst string) (Hops, error) {
	if t.ExecFn == nil {
		t.ExecFn = defaultExecFn
	}

	args := []string{"-q", "1"}
	if t.MaxHops > 0 {
		args = append(args, "-m", strconv.Itoa(t.MaxHops))
	}
	if t.Timeout > 0 {
		args = append(args, "-w", strconv.Itoa(int(t.Timeout.Seconds())))
	}
	args = append(args, dst)

	output, err := t.ExecFn("traceroute", args...)
	if err != nil {
		return nil, fmt.Errorf("error running traceroute: %w", err)
	}

	output = strings.TrimSpace(output)
	lines := strings.Split(output, "\n")
	hops := make([]Hop, 0, len(lines)-1)
	for i, line := range strings.Split(output, "\n") {
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

		hopNum, host, ip, rtt := 0, "", "", ""
		if _, err := fmt.Sscanf(line, "%d %s %s %s", &hopNum, &host, &ip, &rtt); err != nil {
			return nil, fmt.Errorf("unable to parse traceroute line %d %q: %w", i, line, err)
		}

		rttDuration, err := time.ParseDuration(rtt)
		if err != nil {
			return nil, fmt.Errorf("unable to parse rtt %q: %w", rtt, err)
		}

		hops = append(hops, Hop{Addr: ip, Name: host, Hop: hopNum, RTT: &rttDuration})
	}

	return hops, nil
}
