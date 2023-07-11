package util

import (
	"os"
)

func IsAirgap() bool {
	return os.Getenv("DISABLE_OUTBOUND_CONNECTIONS") == "true"
}
