package util

import (
	"os"
	"regexp"

	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

func IsAirgap() bool {
	return os.Getenv("DISABLE_OUTBOUND_CONNECTIONS") == "true"
}

func IsDevEnv() bool {
	return os.Getenv("REPLICATED_ENV") == "dev"
}

func IsDevLicense(license *kotsv1beta1.License) bool {
	result, _ := regexp.MatchString(`replicated-app`, license.Spec.Endpoint)
	return result
}

func IsDevModeEnabled() bool {
	return os.Getenv("DEV_ENABLED") == "true"
}
