package util

import "os"

var (
	PodNamespace string = os.Getenv("POD_NAMESPACE")
)
