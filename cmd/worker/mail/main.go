package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	app, err := InitializeApp()
	if err != nil {
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.Logger.Info("starting mail worker", "consumer", "mail-worker-v1")

	if err := app.Worker.Run(ctx); err != nil {
		app.Logger.Error("mail worker failed", "error", err)
	}

	// The worker owns the NATS connection: drain all subscriptions & buffers then close.
	if app.NATS != nil && app.NATS.Conn != nil {
		if drainErr := app.NATS.Conn.Drain(); drainErr != nil {
			app.Logger.Warn("nats drain error", "error", drainErr)
		}
	}
	app.NATS.Close()
	app.Logger.Info("mail worker stopped")

	if err != nil && ctx.Err() == nil {
		os.Exit(1)
	}
}
