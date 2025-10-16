package helius

import (
	"context"
	"testing"
	"time"
)

func TestClientStartReturnsNotImplemented(t *testing.T) {
	cfg := &Config{
		GRPCEndpoint:     "grpc.example.com:443",
		WSEndpoint:       "wss://example.com",
		APIKey:           "secret",
		RequestTimeout:   time.Second,
		ReconnectBackoff: time.Second,
		ReplaySlots:      64,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, errs := client.Start(ctx, 0)

	select {
	case err := <-errs:
		if err != ErrNotImplemented {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected ErrNotImplemented within timeout")
	}
}
