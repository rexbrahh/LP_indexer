package orchestrator

import (
	"context"
	"testing"
)

func TestOrchestratorRunNotImplemented(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 0
	cfg.EndSlot = 0

	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := orch.Run(context.Background()); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}
