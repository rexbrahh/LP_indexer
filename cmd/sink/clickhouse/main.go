package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rexbrahh/lp-indexer/sinks/clickhouse"
)

func main() {
	logger := log.New(os.Stdout, "sink-clickhouse ", log.LstdFlags|log.Lshortfile)

	cfg, err := clickhouse.ServiceConfigFromEnv()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := clickhouse.NewService(ctx, cfg)
	if err != nil {
		logger.Fatalf("init service: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Println("shutdown signal received")
		cancel()
	}()

	if err := svc.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatalf("service run failed: %v", err)
	}
}
