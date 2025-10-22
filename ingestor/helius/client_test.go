package helius

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mr-tron/base58/base58"

	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"

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

func TestClientEmitsTxMetaAndSwap(t *testing.T) {
	cfg := &Config{
		GRPCEndpoint:     "grpc.example.com:443",
		WSEndpoint:       "wss://example.com",
		APIKey:           "secret",
		RequestTimeout:   time.Second,
		ReconnectBackoff: time.Second,
		ReplaySlots:      64,
		ProgramFilters: map[string]string{
			"raydium": ray.ProgramID,
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
				Slot:      456,
				BlockTime: &pb.UnixTimestamp{Timestamp: 1_700_000_123},
			},
		},
	}

	select {
	case fake.updates <- blockMeta:
	case <-time.After(time.Second):
		t.Fatal("failed to enqueue block meta")
	}

	expectUpdate(t, updates, func(u Update) bool { return u.BlockHead != nil })

	fixture := loadRaydiumFixture(t, "swap_tx_1.json")

	tradeRate := uint32(3000)
	configKey := generateAddress(0xAA)
	configUpdate := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: configKey,
					Owner:  mustDecodeBase58(t, ray.ProgramID),
					Data:   buildConfigData(tradeRate),
				},
			},
		},
	}

	select {
	case fake.updates <- configUpdate:
	case <-time.After(time.Second):
		t.Fatal("failed to enqueue config account")
	}

	poolUpdate := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Account{
			Account: &pb.SubscribeUpdateAccount{
				Account: &pb.SubscribeUpdateAccountInfo{
					Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
					Owner:  mustDecodeBase58(t, ray.ProgramID),
					Data:   buildPoolData(configKey),
				},
			},
		},
	}

	select {
	case fake.updates <- poolUpdate:
	case <-time.After(time.Second):
		t.Fatal("failed to enqueue pool account")
	}

	txUpdate := buildRaydiumUpdate(t, fixture)

	select {
	case fake.updates <- txUpdate:
	case <-time.After(time.Second):
		t.Fatal("failed to enqueue transaction")
	}

	metaUpdate := expectUpdate(t, updates, func(u Update) bool { return u.TxMeta != nil })
	if metaUpdate.TxMeta.GetSig() != fixture.Signature {
		t.Fatalf("unexpected signature %s", metaUpdate.TxMeta.GetSig())
	}
	if metaUpdate.TxMeta.GetSlot() != fixture.Slot {
		t.Fatalf("unexpected slot %d", metaUpdate.TxMeta.GetSlot())
	}
	if !metaUpdate.TxMeta.GetSuccess() {
		t.Fatalf("expected successful transaction")
	}

	swapUpdate := expectUpdate(t, updates, func(u Update) bool { return u.Swap != nil })
	ev := swapUpdate.Swap
	if ev.ProgramId != ray.ProgramID {
		t.Fatalf("unexpected program id %s", ev.ProgramId)
	}
	if ev.PoolId != fixture.PoolAddress {
		t.Fatalf("unexpected pool %s", ev.PoolId)
	}
	if ev.BaseIn != fixture.ExpectedAmountIn {
		t.Fatalf("base_in=%d want %d", ev.BaseIn, fixture.ExpectedAmountIn)
	}
	if ev.QuoteOut != fixture.ExpectedAmountOut {
		t.Fatalf("quote_out=%d want %d", ev.QuoteOut, fixture.ExpectedAmountOut)
	}
	expectedFee := tradeRate / 100
	if ev.FeeBps != expectedFee {
		t.Fatalf("fee_bps=%d want %d", ev.FeeBps, expectedFee)
	}

	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("unexpected error from errs channel: %v", err)
		}
	default:
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

func expectUpdate(t *testing.T, ch <-chan Update, predicate func(Update) bool) Update {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case u, ok := <-ch:
			if !ok {
				t.Fatal("updates channel closed")
			}
			if predicate(u) {
				return u
			}
		case <-timeout:
			t.Fatal("timed out waiting for update")
		}
	}
}

type raydiumFixture struct {
	Signature         string `json:"signature"`
	Slot              uint64 `json:"slot"`
	Timestamp         int64  `json:"timestamp"`
	PoolAddress       string `json:"pool_address"`
	MintA             string `json:"mint_a"`
	MintB             string `json:"mint_b"`
	DecimalsA         uint8  `json:"decimals_a"`
	DecimalsB         uint8  `json:"decimals_b"`
	InstructionData   string `json:"instruction_data"`
	PreVaultA         uint64 `json:"pre_vault_a"`
	PostVaultA        uint64 `json:"post_vault_a"`
	PreVaultB         uint64 `json:"pre_vault_b"`
	PostVaultB        uint64 `json:"post_vault_b"`
	ExpectedAmountIn  uint64 `json:"expected_amount_in"`
	ExpectedAmountOut uint64 `json:"expected_amount_out"`
}

func loadRaydiumFixture(t *testing.T, filename string) *raydiumFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	base := filepath.Join(filepath.Dir(file), "..", "..", "decoder", "raydium", "testdata")
	path := filepath.Join(base, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx raydiumFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return &fx
}

func buildRaydiumUpdate(t *testing.T, fx *raydiumFixture) *pb.SubscribeUpdate {
	t.Helper()
	poolBytes := mustDecodeBase58(t, fx.PoolAddress)
	vaultA := generateAddress(0x21)
	vaultB := generateAddress(0x31)
	user := generateAddress(0x45)
	program := mustDecodeBase58(t, ray.ProgramID)

	accounts := [][]byte{
		user,
		poolBytes,
		vaultA,
		vaultB,
		program,
	}

	instrData, err := hex.DecodeString(fx.InstructionData)
	if err != nil {
		t.Fatalf("decode instruction data: %v", err)
	}

	transaction := &pb.Transaction{
		Signatures: [][]byte{mustDecodeBase58(t, fx.Signature)},
		Message: &pb.Message{
			AccountKeys: accounts,
			Instructions: []*pb.CompiledInstruction{
				{
					ProgramIdIndex: 4,
					Accounts:       []byte{1, 2, 3, 0},
					Data:           instrData,
				},
			},
		},
	}

	meta := &pb.TransactionStatusMeta{
		PreTokenBalances: []*pb.TokenBalance{
			{
				AccountIndex: 2,
				Mint:         fx.MintA,
				Owner:        fx.PoolAddress,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   fmt.Sprintf("%d", fx.PreVaultA),
					Decimals: uint32(fx.DecimalsA),
				},
			},
			{
				AccountIndex: 3,
				Mint:         fx.MintB,
				Owner:        fx.PoolAddress,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   fmt.Sprintf("%d", fx.PreVaultB),
					Decimals: uint32(fx.DecimalsB),
				},
			},
		},
		PostTokenBalances: []*pb.TokenBalance{
			{
				AccountIndex: 2,
				Mint:         fx.MintA,
				Owner:        fx.PoolAddress,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   fmt.Sprintf("%d", fx.PostVaultA),
					Decimals: uint32(fx.DecimalsA),
				},
			},
			{
				AccountIndex: 3,
				Mint:         fx.MintB,
				Owner:        fx.PoolAddress,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   fmt.Sprintf("%d", fx.PostVaultB),
					Decimals: uint32(fx.DecimalsB),
				},
			},
		},
	}

	update := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Transaction{
			Transaction: &pb.SubscribeUpdateTransaction{
				Transaction: &pb.SubscribeUpdateTransactionInfo{
					Signature:   mustDecodeBase58(t, fx.Signature),
					Transaction: transaction,
					Meta:        meta,
				},
				Slot: fx.Slot,
			},
		},
	}
	return update
}

func generateAddress(seed byte) []byte {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = seed
	}
	return buf
}

func mustDecodeBase58(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base58.Decode(s)
	if err != nil {
		t.Fatalf("decode base58 %s: %v", s, err)
	}
	return b
}

func buildConfigData(tradeRate uint32) []byte {
	data := make([]byte, 64)
	copy(data, accountDiscriminator("AmmConfig"))
	binary.LittleEndian.PutUint32(data[8+1+2+32+4:], tradeRate)
	return data
}

func buildPoolData(config []byte) []byte {
	data := make([]byte, 128)
	copy(data, accountDiscriminator("PoolState"))
	copy(data[8+1:], config)
	return data
}

func accountDiscriminator(name string) []byte {
	sum := sha256.Sum256([]byte("account:" + name))
	return sum[:8]
}
