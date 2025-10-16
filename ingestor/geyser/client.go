package geyser

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/rpcpool/yellowstone-grpc/yellowstone-grpc-proto/golang/yellowstone-grpc"
)

const (
	// ReplaySlotWindow defines how many slots to replay on reconnect
	ReplaySlotWindow = 64
	// ReconnectBackoff is the delay between reconnect attempts
	ReconnectBackoff = 5 * time.Second
)

// Client wraps a Yellowstone Geyser gRPC connection with automatic reconnection
type Client struct {
	cfg    *Config
	conn   *grpc.ClientConn
	client pb.GeyserClient
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new Geyser client with the provided configuration
func NewClient(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Connect establishes the gRPC connection to the Geyser endpoint
func (c *Client) Connect() error {
	// For demo purposes using insecure credentials
	// In production, use proper TLS credentials
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(128 * 1024 * 1024), // 128MB max message size
		),
	}

	conn, err := grpc.DialContext(c.ctx, c.cfg.Endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial geyser: %w", err)
	}

	c.conn = conn
	c.client = pb.NewGeyserClient(conn)
	return nil
}

// Subscribe creates a subscription to the Geyser stream with the configured filters
func (c *Client) Subscribe(startSlot uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
	updateCh := make(chan *pb.SubscribeUpdate, 100)
	errCh := make(chan error, 1)

	go c.subscribeLoop(startSlot, updateCh, errCh)

	return updateCh, errCh
}

// subscribeLoop handles the subscription lifecycle with automatic reconnection
func (c *Client) subscribeLoop(startSlot uint64, updateCh chan<- *pb.SubscribeUpdate, errCh chan<- error) {
	defer close(updateCh)
	defer close(errCh)

	currentSlot := startSlot

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Calculate replay slot (current - 64 slots for safety)
		replaySlot := currentSlot
		if currentSlot > ReplaySlotWindow {
			replaySlot = currentSlot - ReplaySlotWindow
		}

		log.Printf("Starting Geyser subscription from slot %d (replay from %d)", currentSlot, replaySlot)

		// Build subscription request
		req := c.buildSubscribeRequest(replaySlot)

		// Create subscription stream
		stream, err := c.client.Subscribe(c.ctx)
		if err != nil {
			log.Printf("Failed to create subscription: %v", err)
			errCh <- fmt.Errorf("subscribe failed: %w", err)

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(ReconnectBackoff):
				continue
			}
		}

		// Send subscribe request
		if err := stream.Send(req); err != nil {
			log.Printf("Failed to send subscribe request: %v", err)
			errCh <- fmt.Errorf("send request failed: %w", err)

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(ReconnectBackoff):
				continue
			}
		}

		// Process stream messages
		lastSlot := c.processStream(stream, updateCh, errCh)
		if lastSlot > currentSlot {
			currentSlot = lastSlot
		}

		log.Printf("Stream ended at slot %d, reconnecting...", currentSlot)

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(ReconnectBackoff):
			// Continue to reconnect
		}
	}
}

// buildSubscribeRequest constructs the subscription request with program filters
func (c *Client) buildSubscribeRequest(startSlot uint64) *pb.SubscribeRequest {
	accounts := make(map[string]*pb.SubscribeRequestFilterAccounts)

	// Add account filters for each configured program
	for name, programID := range c.cfg.ProgramFilters {
		accounts[name] = &pb.SubscribeRequestFilterAccounts{
			Account: []string{},
			Owner:   []string{programID},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{},
		}
	}

	return &pb.SubscribeRequest{
		Slots:              map[string]*pb.SubscribeRequestFilterSlots{
			"client": {},
		},
		Accounts:           accounts,
		Transactions:       map[string]*pb.SubscribeRequestFilterTransactions{},
		TransactionsStatus: map[string]*pb.SubscribeRequestFilterTransactions{},
		Entry:              map[string]*pb.SubscribeRequestFilterEntry{},
		Blocks:             map[string]*pb.SubscribeRequestFilterBlocks{},
		BlocksMeta:         map[string]*pb.SubscribeRequestFilterBlocksMeta{
			"client": {},
		},
		AccountsDataSlice:  []*pb.SubscribeRequestAccountsDataSlice{},
		Ping:               nil,
		Commitment:         pb.CommitmentLevel_CONFIRMED,
	}
}

// processStream reads messages from the stream and forwards them to the update channel
func (c *Client) processStream(stream pb.Geyser_SubscribeClient, updateCh chan<- *pb.SubscribeUpdate, errCh chan<- error) uint64 {
	var lastSlot uint64

	for {
		select {
		case <-c.ctx.Done():
			return lastSlot
		default:
		}

		update, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server")
			return lastSlot
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			errCh <- fmt.Errorf("stream recv failed: %w", err)
			return lastSlot
		}

		// Track latest slot from updates
		if update.Filters != nil && len(update.Filters) > 0 {
			// Update filters indicate the subscription is active
		}

		// Extract slot number from update
		slot := extractSlotFromUpdate(update)
		if slot > lastSlot {
			lastSlot = slot
		}

		// Forward update to channel
		select {
		case updateCh <- update:
		case <-c.ctx.Done():
			return lastSlot
		}
	}
}

// extractSlotFromUpdate extracts the slot number from various update types
func extractSlotFromUpdate(update *pb.SubscribeUpdate) uint64 {
	switch u := update.UpdateOneof.(type) {
	case *pb.SubscribeUpdate_Slot:
		return u.Slot.Slot
	case *pb.SubscribeUpdate_Account:
		return u.Account.Slot
	case *pb.SubscribeUpdate_Transaction:
		return u.Transaction.Slot
	case *pb.SubscribeUpdate_Block:
		return u.Block.Slot
	case *pb.SubscribeUpdate_BlockMeta:
		return u.BlockMeta.Slot
	default:
		return 0
	}
}

// Close gracefully shuts down the client
func (c *Client) Close() error {
	c.cancel()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
