package littleid

import (
	"strings"

	"github.com/google/uuid"
)

func New() string {
	return strings.Split(uuid.New().String(), "-")[1]
}
