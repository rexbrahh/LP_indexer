package natsx

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

func TestPublisherPublishesMessages(t *testing.T) {
	srv, url := runJetStream(t)
	defer srv.Shutdown()

	ensureStream(t, url, "DEX", []string{"dex.sol.>"})

	cfg := DefaultConfig()
	cfg.URL = url
	cfg.Stream = "DEX"
	cfg.SubjectRoot = "dex.sol"
	cfg.PublishTimeout = 2 * time.Second

	pub, err := NewPublisher(cfg)
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	defer pub.Close()

	ctx := context.Background()

	swap := &dexv1.SwapEvent{
		ChainId:   501,
		Slot:      123,
		Sig:       "sig123",
		Index:     1,
		ProgramId: "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK",
		PoolId:    "pool1",
	}
	if err := pub.PublishSwap(ctx, swap); err != nil {
		t.Fatalf("PublishSwap() error = %v", err)
	}

	js := jetStreamContext(t, url)
	msg := getLastMsg(t, js, "DEX", "dex.sol.raydium.swap")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:123:sig123:1" {
		t.Fatalf("unexpected msg id %q", got)
	}
	var decodedSwap dexv1.SwapEvent
	if err := proto.Unmarshal(msg.Data, &decodedSwap); err != nil {
		t.Fatalf("unmarshal swap: %v", err)
	}
	if decodedSwap.GetSig() != swap.GetSig() {
		t.Fatalf("expected sig %s, got %s", swap.GetSig(), decodedSwap.GetSig())
	}

	candlePair := &dexv1.Candle{
		ChainId:     501,
		PairId:      "SOL:USDC",
		PoolId:      "",
		Timeframe:   "1m",
		WindowStart: 1700000000,
	}
	if err := pub.PublishCandle(ctx, candlePair); err != nil {
		t.Fatalf("PublishCandle(pair) error = %v", err)
	}
	msg = getLastMsg(t, js, "DEX", "dex.sol.candle.pair.1m")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:SOL:USDC::1700000000:false" {
		t.Fatalf("unexpected candle msg id %q", got)
	}

	candlePool := &dexv1.Candle{
		ChainId:     501,
		PairId:      "SOL:USDC",
		PoolId:      "pool1",
		Timeframe:   "1m",
		WindowStart: 1700000060,
	}
	if err := pub.PublishCandle(ctx, candlePool); err != nil {
		t.Fatalf("PublishCandle(pool) error = %v", err)
	}
	msg = getLastMsg(t, js, "DEX", "dex.sol.candle.pool.1m")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:SOL:USDC:pool1:1700000060:false" {
		t.Fatalf("unexpected candle msg id %q", got)
	}

	snap := &dexv1.PoolSnapshot{
		ChainId: 501,
		Slot:    999,
		PoolId:  "pool1",
	}
	if err := pub.PublishPoolSnapshot(ctx, snap); err != nil {
		t.Fatalf("PublishPoolSnapshot() error = %v", err)
	}
	msg = getLastMsg(t, js, "DEX", "dex.sol.pool.snapshot")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:999:pool1" {
		t.Fatalf("unexpected snapshot msg id %q", got)
	}

	head := &dexv1.BlockHead{
		ChainId: 501,
		Slot:    777,
		Status:  "confirmed",
	}
	if err := pub.PublishBlockHead(ctx, head); err != nil {
		t.Fatalf("PublishBlockHead() error = %v", err)
	}
	msg = getLastMsg(t, js, "DEX", "dex.sol.blocks.head")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:777" {
		t.Fatalf("unexpected block head msg id %q", got)
	}

	txMeta := &dexv1.TxMeta{
		ChainId: 501,
		Slot:    888,
		Sig:     "sig456",
		Success: true,
	}
	if err := pub.PublishTxMeta(ctx, txMeta); err != nil {
		t.Fatalf("PublishTxMeta() error = %v", err)
	}
	msg = getLastMsg(t, js, "DEX", "dex.sol.tx.meta")
	if got := msg.Header.Get("Nats-Msg-Id"); got != "501:888:sig456" {
		t.Fatalf("unexpected tx meta msg id %q", got)
	}

	ctxTimeout, cancel := pub.WithTimeout(context.Background())
	defer cancel()
	if _, ok := ctxTimeout.Deadline(); !ok {
		t.Fatal("expected context with deadline")
	}
}

func runJetStream(t *testing.T) (*server.Server, string) {
	t.Helper()
	opts := &server.Options{JetStream: true, Host: "127.0.0.1", Port: -1, StoreDir: t.TempDir()}
	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		t.Skip("nats-server not ready in sandbox")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, err := nats.Connect(srv.ClientURL())
		if err == nil {
			nc.Close()
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	addr := srv.Addr()
	if srv.ClientURL() == "nats://127.0.0.1:0" {
		srv.Shutdown()
		t.Skip("nats server no port in sandbox")
	}
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		srv.Shutdown()
		t.Fatal("unexpected addr type")
	}
	url := fmt.Sprintf("nats://127.0.0.1:%d", tcpAddr.Port)
	return srv, url
}

func ensureStream(t *testing.T, url, stream string, subjects []string) {
	t.Helper()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect ensure stream: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream ensure stream: %v", err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: stream, Subjects: subjects, Storage: nats.MemoryStorage}); err != nil {
		t.Fatalf("add stream: %v", err)
	}
}

func jetStreamContext(t *testing.T, url string) nats.JetStreamContext {
	t.Helper()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect js ctx: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream ctx: %v", err)
	}
	return js
}

func getLastMsg(t *testing.T, js nats.JetStreamContext, stream, subject string) *nats.RawStreamMsg {
	t.Helper()
	msg, err := js.GetLastMsg(stream, subject)
	if err != nil {
		t.Fatalf("GetLastMsg(%s, %s): %v", stream, subject, err)
	}
	return msg
}
