package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rexbrahh/lp-indexer/ingestor/geyser"
	natsx "github.com/rexbrahh/lp-indexer/sinks/nats"
)

func main() {
	logger := log.New(os.Stdout, "ingestor-geyser ", log.LstdFlags|log.Lshortfile)

	programsPath := os.Getenv("PROGRAMS_YAML_PATH")
	geyserCfg, err := geyser.LoadConfig(programsPath)
	if err != nil {
		logger.Fatalf("load geyser config: %v", err)
	}

	natsCfg, err := natsx.FromEnv()
	if err != nil {
		logger.Fatalf("load nats config: %v", err)
	}

	metricsAddr := os.Getenv("INGESTOR_METRICS_ADDR")

	service, err := geyser.NewService(geyserCfg, natsCfg, metricsAddr)
	if err != nil {
		logger.Fatalf("init service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Println("shutdown signal received")
		cancel()
	}()

	startSlot := uint64(0)
	if err := service.Run(ctx, startSlot); err != nil && err != context.Canceled {
		logger.Fatalf("service run failed: %v", err)
	}

	logger.Println("service stopped")
}
