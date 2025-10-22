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
	proto "google.golang.org/protobuf/proto"

	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
	"github.com/rexbrahh/lp-indexer/ingestor/common"
	poolmeta "github.com/rexbrahh/lp-indexer/ingestor/internal/pools"

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
	if rate, err := poolmeta.DecodeAmmConfig(configData); err != nil || rate != tradeRate {
		t.Fatalf("DecodeAmmConfig mismatch: rate=%d err=%v", rate, err)
	}
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   configData,
		},
	})
	poolData := buildPoolData(configKey)
	if decoded, err := poolmeta.DecodeRaydiumPool(poolData); err != nil || !bytes.Equal(decoded, configKey) {
		t.Fatalf("DecodeRaydiumPool mismatch err=%v", err)
	}
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   poolData,
		},
	})

	ctx := context.Background()

	blockMeta := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_BlockMeta{
			BlockMeta: &pb.SubscribeUpdateBlockMeta{
				Slot:      fixture.Slot,
				BlockTime: &pb.UnixTimestamp{Timestamp: fixture.Timestamp},
			},
		},
	}
	if err := processor.HandleUpdate(ctx, blockMeta); err != nil {
		t.Fatalf("HandleUpdate block meta: %v", err)
	}

	update := buildRaydiumUpdate(t, fixture)
	if err := processor.HandleUpdate(ctx, update); err != nil {
		t.Fatalf("HandleUpdate transaction returned error: %v", err)
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

	if len(pub.txMetas) != 1 {
		t.Fatalf("expected tx meta to be published")
	}
	meta := pub.txMetas[0]
	if meta.GetSig() != fixture.Signature {
		t.Fatalf("unexpected tx signature %s", meta.GetSig())
	}
	if !meta.GetSuccess() {
		t.Fatal("expected successful tx meta")
	}

	if len(pub.blockHeads) != 1 {
		t.Fatalf("expected 1 block head, got %d", len(pub.blockHeads))
	}
	head := pub.blockHeads[0]
	if head.GetStatus() != "confirmed" {
		t.Fatalf("unexpected block head status %s", head.GetStatus())
	}
}

type stubPublisher struct {
	events     []*dexv1.SwapEvent
	blockHeads []*dexv1.BlockHead
	txMetas    []*dexv1.TxMeta
}

func (s *stubPublisher) PublishSwap(_ context.Context, ev *dexv1.SwapEvent) error {
	clone := proto.Clone(ev).(*dexv1.SwapEvent)
	s.events = append(s.events, clone)
	return nil
}

func (s *stubPublisher) PublishBlockHead(_ context.Context, head *dexv1.BlockHead) error {
	clone := proto.Clone(head).(*dexv1.BlockHead)
	s.blockHeads = append(s.blockHeads, clone)
	return nil
}

func (s *stubPublisher) PublishTxMeta(_ context.Context, meta *dexv1.TxMeta) error {
	clone := proto.Clone(meta).(*dexv1.TxMeta)
	s.txMetas = append(s.txMetas, clone)
	return nil
}

func TestProcessorFinalizesSlotPublishesNonProvisional(t *testing.T) {
	fixture := loadRaydiumFixture(t, "swap_tx_1.json")
	pub := &stubPublisher{}
	cache := common.NewMemorySlotTimeCache()
	cache.Set(fixture.Slot, time.Unix(fixture.Timestamp, 0))
	processor := NewProcessor(pub, cache, nil)
	ctx := context.Background()

	configKey := generateAddress(0xAA)
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   buildConfigData(3000),
		},
	})
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   buildPoolData(configKey),
		},
	})

	processor.HandleUpdate(ctx, &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_BlockMeta{
			BlockMeta: &pb.SubscribeUpdateBlockMeta{
				Slot:      fixture.Slot,
				BlockTime: &pb.UnixTimestamp{Timestamp: fixture.Timestamp},
			},
		},
	})

	if err := processor.HandleUpdate(ctx, buildRaydiumUpdate(t, fixture)); err != nil {
		t.Fatalf("HandleUpdate transaction: %v", err)
	}

	finalize := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot:   fixture.Slot,
				Status: pb.SlotStatus_SLOT_FINALIZED,
			},
		},
	}
	if err := processor.HandleUpdate(ctx, finalize); err != nil {
		t.Fatalf("HandleUpdate finalize: %v", err)
	}

	if len(pub.events) != 2 {
		t.Fatalf("expected two published swaps, got %d", len(pub.events))
	}
	final := pub.events[1]
	if final.Provisional {
		t.Fatalf("finalized event should not be provisional")
	}
	if final.IsUndo {
		t.Fatalf("finalized event should not be undo")
	}

	if len(pub.blockHeads) != 2 {
		t.Fatalf("expected block head finalize publish, got %d", len(pub.blockHeads))
	}
	if pub.blockHeads[1].GetStatus() != "finalized" {
		t.Fatalf("unexpected finalized block head status %s", pub.blockHeads[1].GetStatus())
	}
}

func TestProcessorEmitsUndoOnDeadSlot(t *testing.T) {
	fixture := loadRaydiumFixture(t, "swap_tx_1.json")
	pub := &stubPublisher{}
	cache := common.NewMemorySlotTimeCache()
	cache.Set(fixture.Slot, time.Unix(fixture.Timestamp, 0))
	processor := NewProcessor(pub, cache, nil)
	ctx := context.Background()

	configKey := generateAddress(0xAA)
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   buildConfigData(3000),
		},
	})
	processor.handleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   buildPoolData(configKey),
		},
	})
	processor.HandleUpdate(ctx, &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_BlockMeta{
			BlockMeta: &pb.SubscribeUpdateBlockMeta{
				Slot:      fixture.Slot,
				BlockTime: &pb.UnixTimestamp{Timestamp: fixture.Timestamp},
			},
		},
	})
	processor.HandleUpdate(ctx, buildRaydiumUpdate(t, fixture))

	dead := &pb.SubscribeUpdate{
		UpdateOneof: &pb.SubscribeUpdate_Slot{
			Slot: &pb.SubscribeUpdateSlot{
				Slot:   fixture.Slot,
				Status: pb.SlotStatus_SLOT_DEAD,
			},
		},
	}
	if err := processor.HandleUpdate(ctx, dead); err != nil {
		t.Fatalf("HandleUpdate dead slot: %v", err)
	}

	if len(pub.events) != 2 {
		t.Fatalf("expected undo publish, got %d events", len(pub.events))
	}
	undo := pub.events[1]
	if undo.Provisional {
		t.Fatalf("undo event should not be provisional")
	}
	if !undo.IsUndo {
		t.Fatalf("expected undo flag on dead slot swap")
	}

	if len(pub.blockHeads) != 2 {
		t.Fatalf("expected block head dead publish, got %d", len(pub.blockHeads))
	}
	if pub.blockHeads[1].GetStatus() != "dead" {
		t.Fatalf("unexpected block head status %s", pub.blockHeads[1].GetStatus())
	}
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
