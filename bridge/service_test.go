package bridge

import (
	"context"
	"testing"
)

func TestServiceRunNotImplemented(t *testing.T) {
	cfg := Config{
		SourceURL:    "nats://source:4222",
		TargetURL:    "nats://target:4222",
		SourceStream: "DEX",
		TargetStream: "legacy",
	}
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.Run(context.Background()); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}
