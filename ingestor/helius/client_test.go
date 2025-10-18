package helius

import (
	"context"
	"testing"
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func TestClientPublishesUpdatesAndTracksHealth(t *testing.T) {
	cfg := &Config{
		GRPCEndpoint:     "grpc.example.com:443",
		WSEndpoint:       "wss://example.com",
		APIKey:           "secret",
		RequestTimeout:   time.Second,
		ReconnectBackoff: time.Second,
		ReplaySlots:      64,
		ProgramFilters: map[string]string{
			"raydium": "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	fake := newFakeStreamClient()
	client.newStream = func(*Config) (streamClient, error) {
		return fake, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates, errs := client.Start(ctx, 0)

	blockMeta := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_BlockMeta{
			BlockMeta: &pb.SubscribeUpdateBlockMeta{
				Slot:      123,
				BlockTime: &pb.UnixTimestamp{Timestamp: 1_700_000_000},
			},
		},
	}

	select {
	case fake.updates <- blockMeta:
	case <-time.After(time.Second):
		t.Fatal("failed to enqueue fake block meta update")
	}

	select {
	case u := <-updates:
		if u.BlockHead == nil {
			t.Fatal("expected block head update")
		}
		if u.BlockHead.GetSlot() != 123 {
			t.Fatalf("unexpected slot %d", u.BlockHead.GetSlot())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for block update")
	}

	health := client.Health()
	if !health.Healthy {
		t.Fatal("expected client to be marked healthy")
	}
	if health.LastSlot != 123 {
		t.Fatalf("unexpected last slot %d", health.LastSlot)
	}
	if health.Source != "grpc" {
		t.Fatalf("unexpected source %s", health.Source)
	}

	cancel()

	select {
	case err := <-errs:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected error from errs channel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for errs channel to drain")
	}

	if fake.connectCount == 0 {
		t.Fatal("expected stream client to connect")
	}
	if fake.closeCount == 0 {
		t.Fatal("expected stream client to close")
	}
}

type fakeStreamClient struct {
	updates      chan *pb.SubscribeUpdate
	errs         chan error
	connectCount int
	closeCount   int
}

func newFakeStreamClient() *fakeStreamClient {
	return &fakeStreamClient{
		updates: make(chan *pb.SubscribeUpdate, 8),
		errs:    make(chan error, 1),
	}
}

func (f *fakeStreamClient) Connect() error {
	f.connectCount++
	return nil
}

func (f *fakeStreamClient) Subscribe(uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
	return f.updates, f.errs
}

func (f *fakeStreamClient) Close() error {
	f.closeCount++
	return nil
}
