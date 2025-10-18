package helius

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type apiKeyAuth struct {
	key string
}

func (a apiKeyAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"x-api-key": a.key}, nil
}

func (apiKeyAuth) RequireTransportSecurity() bool { return true }

// StreamClient implements the geyser ClientInterface for the Helius LaserStream
// gRPC endpoint, allowing it to be used with the failover service.
type StreamClient struct {
	cfg    *Config
	conn   *grpc.ClientConn
	client pb.GeyserClient
	ctx    context.Context
	cancel context.CancelFunc
}

// NewStreamClient validates configuration and prepares the LaserStream client.
func NewStreamClient(cfg *Config) (*StreamClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid helius config: %w", err)
	}
	if len(cfg.ProgramFilters) == 0 {
		return nil, fmt.Errorf("ProgramFilters must contain at least one entry")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &StreamClient{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Connect establishes the underlying gRPC connection.
func (c *StreamClient) Connect() error {
	if c.conn != nil {
		return nil
	}
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024 * 1024 * 1024),
		),
		grpc.WithPerRPCCredentials(apiKeyAuth{key: c.cfg.APIKey}),
	}

	conn, err := grpc.DialContext(c.ctx, c.cfg.GRPCEndpoint, options...)
	if err != nil {
		return fmt.Errorf("dial helius lasers tream: %w", err)
	}
	c.conn = conn
	c.client = pb.NewGeyserClient(conn)
	return nil
}

// Subscribe consumes LaserStream updates beginning at startSlot.
func (c *StreamClient) Subscribe(startSlot uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
	updateCh := make(chan *pb.SubscribeUpdate, 128)
	errCh := make(chan error, 1)

	go c.subscribeLoop(startSlot, updateCh, errCh)
	return updateCh, errCh
}

func (c *StreamClient) subscribeLoop(startSlot uint64, updates chan<- *pb.SubscribeUpdate, errs chan<- error) {
	defer close(updates)
	defer close(errs)

	currentSlot := startSlot

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		replaySlot := currentSlot
		if currentSlot > c.cfg.ReplaySlots {
			replaySlot = currentSlot - c.cfg.ReplaySlots
		}

		log.Printf("Helius LaserStream subscribing from slot %d (replay from %d)", currentSlot, replaySlot)

		req := c.buildSubscribeRequest(replaySlot)

		stream, err := c.client.Subscribe(c.ctx)
		if err != nil {
			errs <- fmt.Errorf("helius subscribe failed: %w", err)
			time.Sleep(c.cfg.ReconnectBackoff)
			continue
		}

		if err := stream.Send(req); err != nil {
			errs <- fmt.Errorf("helius send subscribe request failed: %w", err)
			time.Sleep(c.cfg.ReconnectBackoff)
			continue
		}

		lastSlot := c.processStream(stream, updates, errs)
		if lastSlot > currentSlot {
			currentSlot = lastSlot
		}
		log.Printf("Helius stream ended at slot %d, reconnecting", currentSlot)
		time.Sleep(c.cfg.ReconnectBackoff)
	}
}

func (c *StreamClient) buildSubscribeRequest(startSlot uint64) *pb.SubscribeRequest {
	accounts := make(map[string]*pb.SubscribeRequestFilterAccounts)
	for name, programID := range c.cfg.ProgramFilters {
		accounts[name] = &pb.SubscribeRequestFilterAccounts{
			Account: []string{},
			Owner:   []string{programID},
			Filters: []*pb.SubscribeRequestFilterAccountsFilter{},
		}
	}

	programIDs := make([]string, 0, len(c.cfg.ProgramFilters))
	for _, programID := range c.cfg.ProgramFilters {
		programIDs = append(programIDs, programID)
	}

	transactions := map[string]*pb.SubscribeRequestFilterTransactions{
		"programs": {
			AccountInclude: programIDs,
		},
	}

	commitment := pb.CommitmentLevel_CONFIRMED
	return &pb.SubscribeRequest{
		Slots: map[string]*pb.SubscribeRequestFilterSlots{
			"client": {},
		},
		Accounts:           accounts,
		Transactions:       transactions,
		TransactionsStatus: map[string]*pb.SubscribeRequestFilterTransactions{},
		Entry:              map[string]*pb.SubscribeRequestFilterEntry{},
		Blocks:             map[string]*pb.SubscribeRequestFilterBlocks{},
		BlocksMeta: map[string]*pb.SubscribeRequestFilterBlocksMeta{
			"client": {},
		},
		AccountsDataSlice: []*pb.SubscribeRequestAccountsDataSlice{},
		Commitment:        &commitment,
		FromSlot:          &startSlot,
	}
}

func (c *StreamClient) processStream(stream pb.Geyser_SubscribeClient, updates chan<- *pb.SubscribeUpdate, errs chan<- error) uint64 {
	var lastSlot uint64

	for {
		select {
		case <-c.ctx.Done():
			return lastSlot
		default:
		}

		update, err := stream.Recv()
		if err == io.EOF {
			return lastSlot
		}
		if err != nil {
			errs <- fmt.Errorf("helius stream recv failed: %w", err)
			return lastSlot
		}

		slot := extractSlot(update)
		if slot > lastSlot {
			lastSlot = slot
		}

		select {
		case updates <- update:
		case <-c.ctx.Done():
			return lastSlot
		}
	}
}

func extractSlot(update *pb.SubscribeUpdate) uint64 {
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

func (c *StreamClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *StreamClient) Name() string { return "helius" }
