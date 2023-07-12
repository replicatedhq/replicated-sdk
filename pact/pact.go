package pact

import (
	"os"
	"path"

	"github.com/pact-foundation/pact-go/dsl"
)

var (
	pact dsl.Pact
)

func createPact() dsl.Pact {
	dir, _ := os.Getwd()

	pactDir := path.Join(dir, "..", "pacts")
	logDir := path.Join(dir, "..", "pact_logs")

	return dsl.Pact{
		Consumer: "replicated-sdk",
		Provider: "replicated-app",
		LogDir:   logDir,
		PactDir:  pactDir,
		LogLevel: "debug",
	}
}
