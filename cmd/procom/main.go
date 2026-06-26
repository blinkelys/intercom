package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"procom/internal/bootstrap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logger := log.New(os.Stdout, "procom ", log.LstdFlags|log.Lmicroseconds)

	runtimeBundle, err := bootstrap.NewRuntime(logger)
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}

	if err := runtimeBundle.App.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
