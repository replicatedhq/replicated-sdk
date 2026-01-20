package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Ensure all long-running components (HTTP server, leader election, informers, etc.)
	// receive a cancellable context that is cancelled on SIGTERM/SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := RootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
