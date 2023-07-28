package main

import (
	"os"
)

func main() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
