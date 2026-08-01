package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/flidai/leapview/rtest/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], cli.IO{}))
}
