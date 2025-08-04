package exectraceroute

import (
	"bytes"
	"fmt"
	"text/tabwriter"
	"time"
)

// tabwriter setting.
const (
	minWidth int  = 2
	tabWidth int  = 4
	padding  int  = 2
	padChar  byte = ' '
	flags    uint = 0
)

type Hop struct {
	Addr string
	Name string
	Hop  int
	RTT  *time.Duration
}

func (h Hop) String() string {
	return fmt.Sprintf("%d %s %s %s", h.Hop, h.Addr, h.Name, h.RTT)
}

type Hops []Hop

func (hs Hops) String() string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, minWidth, tabWidth, padding, padChar, flags)
	for _, h := range hs {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t\n", h.Hop, h.Name, h.Addr, h.RTT)
	}
	w.Flush()
	return buf.String()
}
