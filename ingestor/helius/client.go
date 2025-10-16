package helius

import (
	"context"
	"errors"
	"sync"
	"time"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

// ErrNotImplemented is emitted until the LaserStream / WebSocket integrations
// are wired up. Downstream callers can treat this as a transient failure and
// remain on the primary ingestor.
var ErrNotImplemented = errors.New("helius LaserStream client not yet implemented")

// Update is a canonical container for the protobuf contracts emitted by the
// fallback ingestor. Exactly one of the pointer fields should be non-nil.
type Update struct {
	BlockHead *dexv1.BlockHead
	TxMeta    *dexv1.TxMeta
	Swap      *dexv1.SwapEvent
}

// HealthSnapshot captures the coarse health signals consumed by the failover
// controller.
type HealthSnapshot struct {
	// LastHeartbeat is the timestamp of the most recent successful message.
	LastHeartbeat time.Time
	// LastSlot indicates the latest slot observed from Helius.
	LastSlot uint64
	// Healthy signals whether the ingestor is currently considered live.
	Healthy bool
	// Source indicates which channel (grpc or websocket) is producing updates.
	Source string
}

// Client manages the lifecycle of Helius connections (LaserStream + WebSocket)
// and exposes a single stream of canonical updates.
type Client struct {
	cfg *Config

	mu     sync.RWMutex
	health HealthSnapshot

	cancel context.CancelFunc
}

// NewClient validates the configuration and prepares a Helius client.
func NewClient(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		health: HealthSnapshot{
			Healthy: false,
		},
	}, nil
}

// Start begins streaming updates starting from the provided slot. The function
// returns immediately with read-only channels backed by an internal goroutine.
func (c *Client) Start(ctx context.Context, startSlot uint64) (<-chan Update, <-chan error) {
	updates := make(chan Update, 128)
	errs := make(chan error, 1)

	runCtx, cancel := context.WithCancel(ctx)
	c.setCancel(cancel)

	go c.run(runCtx, startSlot, updates, errs)
	return updates, errs
}

// Close stops the client and releases resources. It is safe to call multiple
// times.
func (c *Client) Close() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
}

// Health returns a copy of the current health snapshot.
func (c *Client) Health() HealthSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.health
}

func (c *Client) setHealth(fn func(*HealthSnapshot)) {
	c.mu.Lock()
	fn(&c.health)
	c.mu.Unlock()
}

func (c *Client) setCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()
}

// run manages the lifecycle of the LaserStream and WebSocket connections.
func (c *Client) run(ctx context.Context, startSlot uint64, updates chan<- Update, errs chan<- error) {
	defer close(updates)
	defer close(errs)

	// Mark the client as unhealthy until a transport is wired in.
	c.setHealth(func(s *HealthSnapshot) {
		s.Healthy = false
		s.Source = "uninitialised"
	})

	// TODO: Implement LaserStream dialling, message decoding, replay, and
	// websocket fallback. For now we emit a sentinel error so callers keep the
	// primary source active.
	select {
	case <-ctx.Done():
		return
	default:
		errs <- ErrNotImplemented
	}
}
