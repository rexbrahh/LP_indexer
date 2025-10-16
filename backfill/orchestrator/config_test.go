package orchestrator

import "testing"

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 100
	cfg.EndSlot = 200
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestConfigInvalidRange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartSlot = 200
	cfg.EndSlot = 100
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid slot range")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(envStartSlot, "1000")
	t.Setenv(envEndSlot, "2000")
	t.Setenv(envBatchSize, "500")
	t.Setenv(envConcurrency, "2")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.StartSlot != 1000 || cfg.EndSlot != 2000 {
		t.Fatalf("unexpected slot range: %d-%d", cfg.StartSlot, cfg.EndSlot)
	}
	if cfg.BatchSize != 500 {
		t.Fatalf("unexpected batch size %d", cfg.BatchSize)
	}
	if cfg.Concurrency != 2 {
		t.Fatalf("unexpected concurrency %d", cfg.Concurrency)
	}
}
