package parquet

import (
	"context"
	"testing"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

func TestWriterNotImplemented(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Endpoint = "http://minio:9000"
	cfg.Bucket = "dex-parquet"
	cfg.AccessKey = "access"
	cfg.SecretKey = "secret"

	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	ctx := context.Background()
	if err := w.AppendSwap(ctx, &dexv1.SwapEvent{}); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	if err := w.AppendCandle(ctx, &dexv1.Candle{}); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
	if err := w.Flush(ctx); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}
