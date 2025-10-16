package natsx

import (
	"context"
	"errors"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

// ErrNotImplemented is returned until JetStream wiring is added.
var ErrNotImplemented = errors.New("nats publisher not yet implemented")

// Publisher defines the methods required to emit canonical protobuf messages.
type Publisher struct {
	cfg Config
}

// NewPublisher validates configuration and prepares a Publisher instance.
func NewPublisher(cfg Config) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Publisher{cfg: cfg}, nil
}

// PublishSwap publishes a SwapEvent to the appropriate subject.
func (p *Publisher) PublishSwap(ctx context.Context, event *dexv1.SwapEvent) error {
	return p.publish(ctx, event)
}

// PublishPoolSnapshot publishes a PoolSnapshot.
func (p *Publisher) PublishPoolSnapshot(ctx context.Context, snap *dexv1.PoolSnapshot) error {
	return p.publish(ctx, snap)
}

// PublishCandle publishes a Candle update.
func (p *Publisher) PublishCandle(ctx context.Context, candle *dexv1.Candle) error {
	return p.publish(ctx, candle)
}

func (p *Publisher) publish(ctx context.Context, payload any) error {
	_ = ctx
	_ = payload
	return ErrNotImplemented
}

// WithTimeout returns a context with the publisher's timeout applied.
func (p *Publisher) WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := p.cfg.PublishTimeout
	if timeout <= 0 {
		timeout = defaultPublishTimeout
	}
	return context.WithTimeout(parent, timeout)
}

// Config exposes a copy of the publisher configuration.
func (p *Publisher) Config() Config {
	return p.cfg
}
