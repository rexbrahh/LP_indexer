package geyser

import (
	"bytes"
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
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
	"github.com/rexbrahh/lp-indexer/ingestor/common"
	"github.com/rexbrahh/lp-indexer/ingestor/geyser/internal"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

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

func TestProcessorPublishesRaydiumSwap(t *testing.T) {
	fixture := loadRaydiumFixture(t, "swap_tx_1.json")
	pub := &stubPublisher{}
	cache := common.NewMemorySlotTimeCache()
	cache.Set(fixture.Slot, time.Unix(fixture.Timestamp, 0))
	processor := NewProcessor(pub, cache, nil)

	tradeRate := uint32(3000)
	configKey := generateAddress(0xAA)
	configData := buildConfigData(tradeRate)
	if rate, err := internal.DecodeAmmConfig(configData); err != nil || rate != tradeRate {
		t.Fatalf("DecodeAmmConfig mismatch: rate=%d err=%v", rate, err)
	}
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   configData,
		},
	})
	configStr := base58.Encode(configKey)
	if processor.configFees[configStr] == 0 {
		t.Fatalf("expected config fee cache to be populated")
	}
	poolData := buildPoolData(configKey)
	if decoded, err := internal.DecodeRaydiumPool(poolData); err != nil || !bytes.Equal(decoded, configKey) {
		t.Fatalf("DecodeRaydiumPool mismatch err=%v", err)
	}
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   poolData,
		},
	})
	if processor.poolFees[fixture.PoolAddress] == 0 {
		t.Fatalf("expected pool fee cache to be populated")
	}

	update := buildRaydiumUpdate(t, fixture)

	if err := processor.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate returned error: %v", err)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 swap event, got %d", len(pub.events))
	}
	event := pub.events[0]

	if event.BaseIn != fixture.ExpectedAmountIn {
		t.Fatalf("base_in=%d want %d", event.BaseIn, fixture.ExpectedAmountIn)
	}
	if event.QuoteOut != fixture.ExpectedAmountOut {
		t.Fatalf("quote_out=%d want %d", event.QuoteOut, fixture.ExpectedAmountOut)
	}
	if event.ProgramId != "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK" {
		t.Fatalf("unexpected program id %s", event.ProgramId)
	}
	if event.PoolId != fixture.PoolAddress {
		t.Fatalf("pool_id=%s want %s", event.PoolId, fixture.PoolAddress)
	}
	if event.MintBase != fixture.MintA || event.MintQuote != fixture.MintB {
		t.Fatalf("unexpected mint pairing %s/%s", event.MintBase, event.MintQuote)
	}
	if !event.Provisional {
		t.Fatalf("expected provisional flag")
	}
	expectedFee := tradeRate / 100
	if event.FeeBps != expectedFee {
		t.Fatalf("fee_bps=%d want %d", event.FeeBps, expectedFee)
	}
}

type stubPublisher struct {
	events []*dexv1.SwapEvent
}

func (s *stubPublisher) PublishSwap(_ context.Context, ev *dexv1.SwapEvent) error {
	clone := *ev
	s.events = append(s.events, &clone)
	return nil
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
	poolBytes := mustDecodeBase58(t, fx.PoolAddress)
	vaultA := generateAddress(0x21)
	vaultB := generateAddress(0x31)
	user := generateAddress(0x45)
	program := mustDecodeBase58(t, "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK")

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
