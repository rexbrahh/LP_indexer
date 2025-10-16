package orchestrator

import (
	"fmt"
	"os"
	"strconv"
)

const (
	envStartSlot   = "BACKFILL_START_SLOT"
	envEndSlot     = "BACKFILL_END_SLOT"
	envBatchSize   = "BACKFILL_BATCH_SIZE"
	envConcurrency = "BACKFILL_CONCURRENCY"
)

// Config captures runtime parameters for the backfill scheduler.
type Config struct {
	StartSlot   uint64
	EndSlot     uint64
	BatchSize   uint64
	Concurrency int
}

// DefaultConfig sets safe defaults for optional fields.
func DefaultConfig() Config {
	return Config{
		BatchSize:   10_000,
		Concurrency: 4,
	}
}

// Validate ensures slot ranges and batch sizes are sane.
func (c Config) Validate() error {
	if c.EndSlot != 0 && c.StartSlot >= c.EndSlot {
		return fmt.Errorf("start slot must be less than end slot")
	}
	if c.BatchSize == 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	return nil
}

// FromEnv builds a Config from environment variables.
func FromEnv() (Config, error) {
	cfg := DefaultConfig()
	if v := os.Getenv(envStartSlot); v != "" {
		slot, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envStartSlot, err)
		}
		cfg.StartSlot = slot
	}
	if v := os.Getenv(envEndSlot); v != "" {
		slot, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envEndSlot, err)
		}
		cfg.EndSlot = slot
	}
	if v := os.Getenv(envBatchSize); v != "" {
		size, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envBatchSize, err)
		}
		cfg.BatchSize = size
	}
	if v := os.Getenv(envConcurrency); v != "" {
		conc, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envConcurrency, err)
		}
		cfg.Concurrency = conc
	}

	return cfg, cfg.Validate()
}
