package natsx

import (
	"context"
	"testing"
	"time"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

func TestPublisherNotImplemented(t *testing.T) {
	cfg := DefaultConfig()
	cfg.URL = "nats://localhost:4222"
	cfg.Stream = "DEX"

	p, err := NewPublisher(cfg)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}

	ctx := context.Background()
	if err := p.PublishSwap(ctx, &dexv1.SwapEvent{}); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	if err := p.PublishPoolSnapshot(ctx, &dexv1.PoolSnapshot{}); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	if err := p.PublishCandle(ctx, &dexv1.Candle{}); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}

	ctxTimeout, cancel := p.WithTimeout(ctx)
	defer cancel()
	if ddl, ok := ctxTimeout.Deadline(); !ok || ddl.Before(time.Now()) {
		t.Fatalf("expected future deadline, got %v", ddl)
	}
}
