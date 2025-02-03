package recorder

import (
	"fmt"
	"os"
	"time"

	"github.com/wafer-bw/jittermon/internal/peer"
)

var _ peer.Recorder = (*CSV)(nil)

type CSV struct{}

func (r CSV) Record(src, dst peer.PeerID, key string, tsm time.Time, dur time.Duration) error {
	fn := fmt.Sprintf("%s.csv", key)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = fmt.Fprintf(f, "%s,%s,%s,%s\n", tsm.Format(time.RFC3339), src, dst, dur); err != nil {
		return err
	}

	return nil
}
