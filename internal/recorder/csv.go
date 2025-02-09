package recorder

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wafer-bw/jittermon/internal/peer"
)

const (
	fileMode os.FileMode = 0644
)

var _ peer.Recorder = (*CSV)(nil)

type CSV struct {
	mu *sync.Mutex
}

func NewCSV() *CSV {
	return &CSV{mu: &sync.Mutex{}}
}

func (r CSV) Record(tsm time.Time, key, src, dst string, dur *time.Duration) error {
	fn := fmt.Sprintf("%s.csv", key)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	if dur != nil {
		if _, err = fmt.Fprintf(f, "%s,%s,%s,%s\n", tsm.Format(time.RFC3339), src, dst, *dur); err != nil {
			return err
		}
	}

	return nil
}
