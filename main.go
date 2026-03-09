package main

import (
	"logtailr/cmd"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Ensure clean shutdown on signals
	signal.Notify(make(chan os.Signal, 1), syscall.SIGINT, syscall.SIGTERM)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
