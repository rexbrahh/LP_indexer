package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/rexbrahh/lp-indexer/bridge"
)

func main() {
	logger := log.New(os.Stdout, "bridge ", log.LstdFlags|log.Lshortfile)

	cfg, err := bridge.FromEnv()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	opts := []bridge.Option{bridge.WithMetricsRegisterer(registry, registry)}
	if cfg.MetricsAddr != "" {
		opts = append(opts, bridge.WithMetricsServer(cfg.MetricsAddr))
	}

	svc, err := bridge.New(cfg, opts...)
	if err != nil {
		logger.Fatalf("init bridge: %v", err)
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

	if err := svc.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatalf("bridge run failed: %v", err)
	}
}
