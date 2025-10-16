package orchestrator

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
)

func TestRunSchedulesExpectedRanges(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 100
	cfg.EndSlot = 130
	cfg.BatchSize = 10
	cfg.Concurrency = 2

	var (
		mu        sync.Mutex
		processed []Range
	)

	processor := func(ctx context.Context, rng Range) error {
		mu.Lock()
		processed = append(processed, rng)
		mu.Unlock()
		return nil
	}

	orch, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := orch.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(processed) != 3 {
		t.Fatalf("expected 3 ranges processed, got %d", len(processed))
	}

	sort.Slice(processed, func(i, j int) bool {
		return processed[i].StartSlot < processed[j].StartSlot
	})

	expected := []Range{
		{StartSlot: 100, EndSlot: 110},
		{StartSlot: 110, EndSlot: 120},
		{StartSlot: 120, EndSlot: 130},
	}

	for i, got := range processed {
		want := expected[i]
		if got != want {
			t.Fatalf("range[%d] mismatch: got %+v want %+v", i, got, want)
		}
	}
}

func TestRunPropagatesProcessorError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 200
	cfg.EndSlot = 240
	cfg.BatchSize = 20
	cfg.Concurrency = 1

	processor := func(ctx context.Context, rng Range) error {
		if rng.StartSlot >= 220 {
			return context.DeadlineExceeded
		}
		return nil
	}

	orch, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = orch.Run(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 0
	cfg.EndSlot = 0 // open-ended
	cfg.BatchSize = 5
	cfg.Concurrency = 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed int
	processor := func(ctx context.Context, rng Range) error {
		processed++
		if processed == 3 {
			cancel()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}

	orch, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = orch.Run(ctx)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if processed < 3 {
		t.Fatalf("expected at least 3 ranges processed before cancel, got %d", processed)
	}
}
