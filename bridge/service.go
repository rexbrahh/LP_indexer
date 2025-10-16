package bridge

import (
	"context"
	"errors"
)

// ErrNotImplemented indicates the bridge logic is pending.
var ErrNotImplemented = errors.New("bridge service not yet implemented")

// Service wires source and target JetStream connections and forwards messages.
type Service struct {
	cfg Config
}

// New creates a Service with validated configuration.
func New(cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Service{cfg: cfg}, nil
}

// Run starts the bridge until the context is cancelled. Implementation TBD.
func (s *Service) Run(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

// Config returns the service configuration.
func (s *Service) Config() Config {
	return s.cfg
}
