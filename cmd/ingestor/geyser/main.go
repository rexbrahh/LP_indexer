package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rexbrahh/lp-indexer/ingestor/geyser"
	"github.com/rexbrahh/lp-indexer/ingestor/helius"
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

	var service interface {
		Run(ctx context.Context, startSlot uint64) error
	}

	if os.Getenv("ENABLE_HELIUS_FALLBACK") == "1" {
		logger.Println("Helius fallback enabled")
		primaryClient, err := geyser.NewClient(geyserCfg)
		if err != nil {
			logger.Fatalf("init geyser client: %v", err)
		}

		heliusCfg, err := helius.FromEnv()
		if err != nil {
			logger.Fatalf("load helius config: %v", err)
		}
		heliusCfg.ProgramFilters = geyserCfg.ProgramFilters

		fallbackClient, err := helius.NewStreamClient(heliusCfg)
		if err != nil {
			logger.Fatalf("init helius client: %v", err)
		}

		service, err = geyser.NewFailoverService(primaryClient, fallbackClient, natsCfg, metricsAddr)
		if err != nil {
			logger.Fatalf("init failover service: %v", err)
		}
	} else {
		svc, err := geyser.NewService(geyserCfg, natsCfg, metricsAddr)
		if err != nil {
			logger.Fatalf("init service: %v", err)
		}
		service = svc
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
