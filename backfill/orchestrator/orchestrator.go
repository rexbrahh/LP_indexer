package orchestrator

import (
	"context"
	"errors"
)

// ErrNotImplemented signals that the orchestrator logic is pending.
var ErrNotImplemented = errors.New("backfill orchestrator not yet implemented")

// Orchestrator coordinates Substreams backfills and sink writes.
type Orchestrator struct {
	cfg Config
}

// New creates an orchestrator with the provided configuration.
func New(cfg Config) (*Orchestrator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Orchestrator{cfg: cfg}, nil
}

// Run executes the backfill process for the configured slot range.
func (o *Orchestrator) Run(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

// Config exposes a copy of the orchestrator config.
func (o *Orchestrator) Config() Config {
	return o.cfg
}
