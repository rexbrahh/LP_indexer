package geyser

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// stubClient implements ClientInterface for failover tests.
type stubClient struct {
	name         string
	connectErr   error
	subscribeFn  func(uint64) (<-chan *pb.SubscribeUpdate, <-chan error)
	closeFn      func() error
	connectCount int
	closeCount   int
}

func (s *stubClient) Connect() error {
	s.connectCount++
	return s.connectErr
}

func (s *stubClient) Subscribe(startSlot uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
	if s.subscribeFn == nil {
		return nil, nil
	}
	return s.subscribeFn(startSlot)
}

func (s *stubClient) Close() error {
	s.closeCount++
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

func (s *stubClient) Name() string { return s.name }

// failoverStubPublisher captures published swaps for assertions.
type failoverStubPublisher struct {
	mu    sync.Mutex
	swaps []*dexv1.SwapEvent
	err   error
}

func (p *failoverStubPublisher) PublishSwap(_ context.Context, ev *dexv1.SwapEvent) error {
	if p.err != nil {
		return p.err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.swaps = append(p.swaps, ev)
	return nil
}

func (p *failoverStubPublisher) PublishBlockHead(context.Context, *dexv1.BlockHead) error {
	return nil
}

func (p *failoverStubPublisher) PublishTxMeta(context.Context, *dexv1.TxMeta) error {
	return nil
}

func TestFailoverServiceSwitchesToFallback(t *testing.T) {
	var fallbackInvoked sync.WaitGroup
	fallbackInvoked.Add(1)

	primary := &stubClient{
		name: "geyser",
		subscribeFn: func(uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
			updates := make(chan *pb.SubscribeUpdate)
			errs := make(chan error, 1)
			go func() {
				defer close(updates)
				defer close(errs)
				errs <- errors.New("primary stream failed")
			}()
			return updates, errs
		},
	}

	fallback := &stubClient{
		name: "helius",
		subscribeFn: func(uint64) (<-chan *pb.SubscribeUpdate, <-chan error) {
			updates := make(chan *pb.SubscribeUpdate, 1)
			errs := make(chan error)
			go func() {
				defer close(updates)
				defer close(errs)
				updates <- &pb.SubscribeUpdate{
					UpdateOneof: &pb.SubscribeUpdate_BlockMeta{
						BlockMeta: &pb.SubscribeUpdateBlockMeta{},
					},
				}
				fallbackInvoked.Done()
			}()
			return updates, errs
		},
	}

	pub := &failoverStubPublisher{}
	proc := NewProcessor(pub, nil, prometheus.NewRegistry())

	svc := &FailoverService{
		primary:            primary,
		fallback:           fallback,
		processor:          proc,
		metrics:            newFailoverMetrics(prometheus.NewRegistry()),
		primaryRetryDelay:  5 * time.Millisecond,
		fallbackRetryDelay: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx, 0)
	}()

	done := make(chan struct{})
	go func() {
		fallbackInvoked.Wait()
		close(done)
	}()

	select {
	case <-done:
		cancel()
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("fallback was not invoked within timeout")
	}

	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}

	if primary.connectCount == 0 {
		t.Fatal("primary client was never connected")
	}
	if fallback.connectCount == 0 {
		t.Fatal("fallback client was never connected")
	}
}
