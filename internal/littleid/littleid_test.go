package littleid_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wafer-bw/jittermon/internal/littleid"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("returns a new littleid", func(t *testing.T) {
		t.Parallel()

		id := littleid.New()
		require.Len(t, id, 4)
	})
}
