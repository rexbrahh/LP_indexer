package helius

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mr-tron/base58/base58"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

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

	cancel    context.CancelFunc
	newStream func(*Config) (streamClient, error)
}

type streamClient interface {
	Connect() error
	Subscribe(startSlot uint64) (<-chan *pb.SubscribeUpdate, <-chan error)
	Close() error
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
		newStream: func(cfg *Config) (streamClient, error) {
			return NewStreamClient(cfg)
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

	stream, err := c.newStream(c.cfg)
	if err != nil {
		select {
		case errs <- fmt.Errorf("init helius stream client: %w", err):
		case <-ctx.Done():
		}
		return
	}
	defer stream.Close()

	if err := stream.Connect(); err != nil {
		select {
		case errs <- fmt.Errorf("connect helius stream: %w", err):
		case <-ctx.Done():
		}
		return
	}

	updateCh, errCh := stream.Subscribe(startSlot)

	c.setHealth(func(s *HealthSnapshot) {
		s.Healthy = true
		s.Source = "grpc"
		s.LastHeartbeat = time.Now()
	})

	for {
		select {
		case <-ctx.Done():
			c.setHealth(func(s *HealthSnapshot) {
				s.Healthy = false
			})
			return
		case err, ok := <-errCh:
			if !ok || err == nil {
				continue
			}
			c.setHealth(func(s *HealthSnapshot) {
				s.Healthy = false
				s.Source = "grpc"
			})
			select {
			case errs <- err:
			default:
			}
		case update, ok := <-updateCh:
			if !ok {
				c.setHealth(func(s *HealthSnapshot) {
					s.Healthy = false
				})
				return
			}
			slot := slotFromUpdate(update)
			c.setHealth(func(s *HealthSnapshot) {
				s.Healthy = true
				s.Source = "grpc"
				s.LastHeartbeat = time.Now()
				if slot > 0 {
					s.LastSlot = slot
				}
			})
			c.emitUpdate(ctx, update, updates)
		}
	}
}

func (c *Client) emitUpdate(ctx context.Context, in *pb.SubscribeUpdate, out chan<- Update) {
	switch u := in.GetUpdateOneof().(type) {
	case *pb.SubscribeUpdate_BlockMeta:
		if u.BlockMeta == nil {
			return
		}
		head := &dexv1.BlockHead{
			ChainId: 501,
			Slot:    u.BlockMeta.GetSlot(),
			Status:  "confirmed",
		}
		if ts := u.BlockMeta.GetBlockTime(); ts != nil {
			head.TsSec = uint64(ts.GetTimestamp())
		}
		c.sendUpdate(ctx, out, Update{BlockHead: head})
	case *pb.SubscribeUpdate_Transaction:
		if update := convertTransaction(u.Transaction); update != nil {
			c.sendUpdate(ctx, out, *update)
		}
	default:
		// Ignore other update types for now.
	}
}

func (c *Client) sendUpdate(ctx context.Context, out chan<- Update, update Update) {
	select {
	case out <- update:
	case <-ctx.Done():
	}
}

func convertTransaction(tx *pb.SubscribeUpdateTransaction) *Update {
	if tx == nil {
		return nil
	}
	info := tx.GetTransaction()
	if info == nil {
		return nil
	}
	meta := info.GetMeta()
	if meta == nil {
		return nil
	}
	signature := base58.Encode(info.GetSignature())
	update := &Update{
		TxMeta: &dexv1.TxMeta{
			ChainId: 501,
			Slot:    tx.GetSlot(),
			Sig:     signature,
			Success: meta.GetErr() == nil,
			CuUsed:  meta.GetComputeUnitsConsumed(),
			CuPrice: 0,
			LogMsgs: meta.GetLogMessages(),
		},
	}
	return update
}

func slotFromUpdate(update *pb.SubscribeUpdate) uint64 {
	switch u := update.GetUpdateOneof().(type) {
	case *pb.SubscribeUpdate_Slot:
		return u.Slot.GetSlot()
	case *pb.SubscribeUpdate_Account:
		return u.Account.GetSlot()
	case *pb.SubscribeUpdate_Transaction:
		return u.Transaction.GetSlot()
	case *pb.SubscribeUpdate_Block:
		return u.Block.GetSlot()
	case *pb.SubscribeUpdate_BlockMeta:
		return u.BlockMeta.GetSlot()
	default:
		return 0
	}
}
