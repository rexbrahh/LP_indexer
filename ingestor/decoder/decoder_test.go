package decoder

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mr-tron/base58/base58"

	orcawhirlpool "github.com/rexbrahh/lp-indexer/decoder/orca_whirlpool"
	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"
	"github.com/rexbrahh/lp-indexer/ingestor/common"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func TestDecoder_DecodeTransaction_Raydium(t *testing.T) {
	fixture := loadRaydiumFixture(t, "swap_tx_1.json")

	cache := common.NewMemorySlotTimeCache()
	cache.Set(fixture.Slot, time.Unix(fixture.Timestamp, 0))
	dec := New(cache)

	tradeRate := uint32(3000)
	configKey := generateAddress(0xAA)
	configData := buildConfigData(tradeRate)
	dec.HandleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   configData,
		},
	})
	poolData := buildPoolData(configKey)
	dec.HandleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: mustDecodeBase58(t, fixture.PoolAddress),
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   poolData,
		},
	})

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      fixture.Slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: fixture.Timestamp},
	})

	tx := buildRaydiumTransaction(t, fixture)
	events, err := dec.DecodeTransaction(tx)
	if err != nil {
		t.Fatalf("DecodeTransaction returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 swap event, got %d", len(events))
	}
	ev := events[0]

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
	if !ev.Provisional {
		t.Fatal("expected provisional flag to be set")
	}
	expectedFee := tradeRate / 100
	if ev.FeeBps != expectedFee {
		t.Fatalf("fee_bps=%d want %d", ev.FeeBps, expectedFee)
	}
	if ev.GetSlot() != fixture.Slot {
		t.Fatalf("unexpected slot %d", ev.GetSlot())
	}
}

func TestDecoder_DecodeTransaction_RaydiumDecodeError(t *testing.T) {
	fixture := loadRaydiumFixture(t, "swap_tx_1.json")

	cache := common.NewMemorySlotTimeCache()
	cache.Set(fixture.Slot, time.Unix(fixture.Timestamp, 0))
	dec := New(cache)

	tradeRate := uint32(3000)
	configKey := generateAddress(0xAB)
	configData := buildConfigData(tradeRate)
	dec.HandleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: configKey,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   configData,
		},
	})
	poolsBytes := mustDecodeBase58(t, fixture.PoolAddress)
	poolData := buildPoolData(configKey)
	dec.HandleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: poolsBytes,
			Owner:  mustDecodeBase58(t, ray.ProgramID),
			Data:   poolData,
		},
	})

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      fixture.Slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: fixture.Timestamp},
	})

	tx := buildRaydiumTransaction(t, fixture)
	info := tx.GetTransaction()
	if info == nil {
		t.Fatal("missing transaction info")
	}
	inner := info.GetTransaction()
	if inner == nil || inner.Message == nil || len(inner.Message.Instructions) == 0 {
		t.Fatal("unexpected transaction shape")
	}
	inner.Message.Instructions[0].Data = []byte{0xFF}

	events, err := dec.DecodeTransaction(tx)
	if err == nil {
		t.Fatal("expected decode error")
	}
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("expected DecodeError, got %T", err)
	}
	if decodeErr.Program != ray.ProgramID {
		t.Fatalf("unexpected program id %s", decodeErr.Program)
	}
	if events != nil {
		t.Fatalf("expected nil events on error, got %d", len(events))
	}
}

func TestDecoder_DecodeTransaction_Orca(t *testing.T) {
	cache := common.NewMemorySlotTimeCache()
	slot := uint64(987654)
	timestamp := int64(1_700_000_500)
	cache.Set(slot, time.Unix(timestamp, 0))

	dec := New(cache)

	poolKey := generateAddress(0x77)
	mintA := generateAddress(0x22)
	mintB := generateAddress(0x33)
	vaultA := generateAddress(0x44)
	vaultB := generateAddress(0x55)

	feeRate := uint16(2500) // 25 bps
	poolData := buildOrcaPoolData(t, mintA, mintB, vaultA, vaultB, feeRate)
	dec.HandleAccount(&pb.SubscribeUpdateAccount{
		Account: &pb.SubscribeUpdateAccountInfo{
			Pubkey: poolKey,
			Owner:  mustDecodeBase58(t, orcawhirlpool.WhirlpoolProgramID),
			Data:   poolData,
		},
	})

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: timestamp},
	})

	tx := buildOrcaTransaction(t, slot, poolKey, mintA, mintB, vaultA, vaultB, feeRate)
	events, err := dec.DecodeTransaction(tx)
	if err != nil {
		t.Fatalf("DecodeTransaction returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 swap event, got %d", len(events))
	}
	ev := events[0]

	poolID := base58.Encode(poolKey)
	mintAStr := base58.Encode(mintA)
	mintBStr := base58.Encode(mintB)

	if ev.ProgramId != orcawhirlpool.WhirlpoolProgramID {
		t.Fatalf("unexpected program id %s", ev.ProgramId)
	}
	if ev.PoolId != poolID {
		t.Fatalf("unexpected pool id %s want %s", ev.PoolId, poolID)
	}
	if ev.MintBase != mintAStr || ev.MintQuote != mintBStr {
		t.Fatalf("unexpected mints %s/%s", ev.MintBase, ev.MintQuote)
	}
	if ev.BaseOut != 500_000 {
		t.Fatalf("base_out=%d want 500000", ev.BaseOut)
	}
	if ev.QuoteIn != 700_000 {
		t.Fatalf("quote_in=%d want 700000", ev.QuoteIn)
	}
	if ev.FeeBps != uint32(feeRate/100) {
		t.Fatalf("fee_bps=%d want %d", ev.FeeBps, feeRate/100)
	}
	if !ev.Provisional {
		t.Fatal("expected provisional flag")
	}
	if ev.DecBase != 6 || ev.DecQuote != 6 {
		t.Fatalf("unexpected decimals base=%d quote=%d", ev.DecBase, ev.DecQuote)
	}
}

func TestDecoder_DecodeTransaction_OrcaMissingPoolMetadata(t *testing.T) {
	cache := common.NewMemorySlotTimeCache()
	slot := uint64(11111)
	timestamp := int64(1_800_000_000)
	cache.Set(slot, time.Unix(timestamp, 0))

	dec := New(cache)

	poolKey := generateAddress(0xEE)
	mintA := generateAddress(0xAA)
	mintB := generateAddress(0xBB)
	vaultA := generateAddress(0xCC)
	vaultB := generateAddress(0xDD)
	feeRate := uint16(2500)

	// Intentionally skip HandleAccount so orca pool metadata is missing.

	tx := buildOrcaTransaction(t, slot, poolKey, mintA, mintB, vaultA, vaultB, feeRate)
	events, err := dec.DecodeTransaction(tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected zero swap events, got %d", len(events))
	}
}

func TestDecoder_DecodeTransaction_MeteoraCPMM(t *testing.T) {
	fx := loadMeteoraFixture(t, "cpmm_swap.json")

	cache := common.NewMemorySlotTimeCache()
	cache.Set(fx.Slot, time.Unix(fx.Timestamp, 0))
	dec := New(cache)

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      fx.Slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: fx.Timestamp},
	})

	tx := buildMeteoraTransaction(t, fx)
	events, err := dec.DecodeTransaction(tx)
	if err != nil {
		t.Fatalf("DecodeTransaction returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 swap event, got %d", len(events))
	}
	ev := events[0]

	if ev.ProgramId != fx.ProgramID {
		t.Fatalf("unexpected program id %s", ev.ProgramId)
	}
	if ev.MintBase != fx.Accounts["mint_base"] {
		t.Fatalf("unexpected base mint %s", ev.MintBase)
	}
	if ev.MintQuote != fx.Accounts["mint_quote"] {
		t.Fatalf("unexpected quote mint %s", ev.MintQuote)
	}
	if fx.Expected.BaseIn > 0 && ev.BaseIn != fx.Expected.BaseIn {
		t.Fatalf("base_in=%d want %d", ev.BaseIn, fx.Expected.BaseIn)
	}
	if fx.Expected.QuoteOut > 0 && ev.QuoteOut != fx.Expected.QuoteOut {
		t.Fatalf("quote_out=%d want %d", ev.QuoteOut, fx.Expected.QuoteOut)
	}
	if ev.Slot != fx.Slot {
		t.Fatalf("unexpected slot %d", ev.Slot)
	}
	if ev.BaseOut != 0 || ev.QuoteIn != 0 {
		t.Fatalf("unexpected output values base_out=%d quote_in=%d", ev.BaseOut, ev.QuoteIn)
	}
	if fx.Expected.FeeBps > 0 && ev.FeeBps != fx.Expected.FeeBps {
		t.Fatalf("fee_bps=%d want %d", ev.FeeBps, fx.Expected.FeeBps)
	}
	if fx.Expected.RealBase > 0 && ev.ReservesBase != fx.Expected.RealBase {
		t.Fatalf("reserves_base=%d want %d", ev.ReservesBase, fx.Expected.RealBase)
	}
	if fx.Expected.RealQuote > 0 && ev.ReservesQuote != fx.Expected.RealQuote {
		t.Fatalf("reserves_quote=%d want %d", ev.ReservesQuote, fx.Expected.RealQuote)
	}
}

func TestDecoder_DecodeTransaction_MeteoraDLMM(t *testing.T) {
	fx := loadMeteoraFixture(t, "dlmm_swap.json")

	cache := common.NewMemorySlotTimeCache()
	cache.Set(fx.Slot, time.Unix(fx.Timestamp, 0))
	dec := New(cache)

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      fx.Slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: fx.Timestamp},
	})

	tx := buildMeteoraTransaction(t, fx)
	events, err := dec.DecodeTransaction(tx)
	if err != nil {
		t.Fatalf("DecodeTransaction returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 swap event, got %d", len(events))
	}
	ev := events[0]

	if ev.ProgramId != fx.ProgramID {
		t.Fatalf("unexpected program id %s", ev.ProgramId)
	}
	if fx.Expected.BaseOut > 0 && ev.BaseOut != fx.Expected.BaseOut {
		t.Fatalf("base_out=%d want %d", ev.BaseOut, fx.Expected.BaseOut)
	}
	if fx.Expected.QuoteIn > 0 && ev.QuoteIn != fx.Expected.QuoteIn {
		t.Fatalf("quote_in=%d want %d", ev.QuoteIn, fx.Expected.QuoteIn)
	}
	if ev.Slot != fx.Slot {
		t.Fatalf("unexpected slot %d", ev.Slot)
	}
	if ev.BaseIn != 0 {
		t.Fatalf("expected zero base_in for quote->base swap, got %d", ev.BaseIn)
	}
	if fx.Expected.FeeBps > 0 && ev.FeeBps != fx.Expected.FeeBps {
		t.Fatalf("fee_bps=%d want %d", ev.FeeBps, fx.Expected.FeeBps)
	}
	if ev.QuoteOut != 0 {
		t.Fatalf("expected zero quote_out for quote->base swap, got %d", ev.QuoteOut)
	}
	if fx.Expected.VirtualBase > 0 && ev.ReservesBase != fx.Expected.VirtualBase {
		t.Fatalf("reserves_base=%d want %d", ev.ReservesBase, fx.Expected.VirtualBase)
	}
	if fx.Expected.VirtualQuote > 0 && ev.ReservesQuote != fx.Expected.VirtualQuote {
		t.Fatalf("reserves_quote=%d want %d", ev.ReservesQuote, fx.Expected.VirtualQuote)
	}
}

func TestDecoder_DecodeTransaction_MeteoraDecodeError(t *testing.T) {
	fx := loadMeteoraFixture(t, "cpmm_swap.json")

	cache := common.NewMemorySlotTimeCache()
	cache.Set(fx.Slot, time.Unix(fx.Timestamp, 0))
	dec := New(cache)

	dec.HandleBlockMeta(&pb.SubscribeUpdateBlockMeta{
		Slot:      fx.Slot,
		BlockTime: &pb.UnixTimestamp{Timestamp: fx.Timestamp},
	})

	tx := buildMeteoraTransaction(t, fx)
	info := tx.GetTransaction()
	if info == nil {
		t.Fatal("missing transaction info")
	}
	meta := info.GetMeta()
	if meta == nil {
		t.Fatal("missing transaction meta")
	}
	meta.PreTokenBalances = nil
	meta.PostTokenBalances = nil

	events, err := dec.DecodeTransaction(tx)
	if err == nil {
		t.Fatal("expected decode error")
	}
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("expected DecodeError, got %T", err)
	}
	if decodeErr.Program != fx.ProgramID {
		t.Fatalf("decode error program=%s want %s", decodeErr.Program, fx.ProgramID)
	}
	if events != nil {
		t.Fatalf("expected nil events when decoding fails, got %d", len(events))
	}
}

// --- Helpers ---

type meteoraFixture struct {
	ProgramID           string                `json:"program_id"`
	Signature           string                `json:"signature"`
	Slot                uint64                `json:"slot"`
	Timestamp           int64                 `json:"timestamp"`
	Accounts            map[string]string     `json:"accounts"`
	AccountOrder        []string              `json:"account_order"`
	InstructionAccounts []int                 `json:"instruction_accounts"`
	Logs                []string              `json:"logs"`
	PreTokenBalances    []meteoraTokenBalance `json:"pre_token_balances"`
	PostTokenBalances   []meteoraTokenBalance `json:"post_token_balances"`
	Expected            meteoraExpected       `json:"expected"`
}

type meteoraTokenBalance struct {
	AccountIndex uint32 `json:"account_index"`
	Mint         string `json:"mint"`
	Owner        string `json:"owner"`
	Amount       string `json:"amount"`
	Decimals     uint32 `json:"decimals"`
}

type meteoraExpected struct {
	BaseIn       uint64 `json:"base_in"`
	BaseOut      uint64 `json:"base_out"`
	QuoteIn      uint64 `json:"quote_in"`
	QuoteOut     uint64 `json:"quote_out"`
	VirtualBase  uint64 `json:"virtual_base"`
	VirtualQuote uint64 `json:"virtual_quote"`
	RealBase     uint64 `json:"real_base"`
	RealQuote    uint64 `json:"real_quote"`
	FeeBps       uint32 `json:"fee_bps"`
}

func loadMeteoraFixture(t *testing.T, filename string) *meteoraFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	base := filepath.Join(filepath.Dir(file), "..", "..", "decoder", "meteora", "testdata")
	data, err := os.ReadFile(filepath.Join(base, filename))
	if err != nil {
		t.Fatalf("read meteora fixture: %v", err)
	}
	var fx meteoraFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("decode meteora fixture: %v", err)
	}
	return &fx
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
	data, err := os.ReadFile(filepath.Join(base, filename))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx raydiumFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return &fx
}

func buildRaydiumTransaction(t *testing.T, fx *raydiumFixture) *pb.SubscribeUpdateTransaction {
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

	return &pb.SubscribeUpdateTransaction{
		Transaction: &pb.SubscribeUpdateTransactionInfo{
			Signature:   mustDecodeBase58(t, fx.Signature),
			Transaction: transaction,
			Meta:        meta,
		},
		Slot: fx.Slot,
	}
}

func buildOrcaTransaction(t *testing.T, slot uint64, poolKey, mintA, mintB, vaultA, vaultB []byte, feeRate uint16) *pb.SubscribeUpdateTransaction {
	t.Helper()

	poolID := base58.Encode(poolKey)
	sig := generateSignature(0x9A)

	accounts := [][]byte{
		generateAddress(0x01), // user
		generateAddress(0x02), // authority
		poolKey,               // pool
		vaultA,                // vault A
		vaultB,                // vault B
		mustDecodeBase58(t, orcawhirlpool.WhirlpoolProgramID), // program id
	}

	instr := &pb.CompiledInstruction{
		ProgramIdIndex: 5,
		Accounts:       []byte{0, 1, 2, 3, 4},
		Data:           []byte{0x0}, // data unused by decoder
	}

	meta := &pb.TransactionStatusMeta{
		PreTokenBalances: []*pb.TokenBalance{
			{
				AccountIndex: 3,
				Mint:         base58.Encode(mintA),
				Owner:        poolID,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   "1000000",
					Decimals: 6,
				},
			},
			{
				AccountIndex: 4,
				Mint:         base58.Encode(mintB),
				Owner:        poolID,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   "500000",
					Decimals: 6,
				},
			},
		},
		PostTokenBalances: []*pb.TokenBalance{
			{
				AccountIndex: 3,
				Mint:         base58.Encode(mintA),
				Owner:        poolID,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   "500000",
					Decimals: 6,
				},
			},
			{
				AccountIndex: 4,
				Mint:         base58.Encode(mintB),
				Owner:        poolID,
				UiTokenAmount: &pb.UiTokenAmount{
					Amount:   "1200000",
					Decimals: 6,
				},
			},
		},
	}

	transaction := &pb.Transaction{
		Signatures: [][]byte{sig},
		Message: &pb.Message{
			AccountKeys: accounts,
			Instructions: []*pb.CompiledInstruction{
				instr,
			},
		},
	}

	return &pb.SubscribeUpdateTransaction{
		Transaction: &pb.SubscribeUpdateTransactionInfo{
			Signature:   sig,
			Transaction: transaction,
			Meta:        meta,
		},
		Slot: slot,
	}
}

func buildMeteoraTransaction(t *testing.T, fx *meteoraFixture) *pb.SubscribeUpdateTransaction {
	t.Helper()

	accountBytes := make(map[string][]byte, len(fx.Accounts)+1)
	for name, value := range fx.Accounts {
		accountBytes[name] = mustDecodeBase58(t, value)
	}
	accountBytes["program"] = mustDecodeBase58(t, fx.ProgramID)

	accountKeys := make([][]byte, len(fx.AccountOrder))
	for i, name := range fx.AccountOrder {
		key, ok := accountBytes[name]
		if !ok {
			t.Fatalf("account %s missing in fixture", name)
		}
		accountKeys[i] = key
	}

	programIndex := len(accountKeys) - 1

	signature := mustDecodeBase58(t, fx.Signature)

	instrAccounts := make([]byte, len(fx.InstructionAccounts))
	for i, idx := range fx.InstructionAccounts {
		if idx < 0 || idx > 255 {
			t.Fatalf("instruction account index %d out of range", idx)
		}
		instrAccounts[i] = byte(idx)
	}

	meta := &pb.TransactionStatusMeta{
		PreTokenBalances:  buildPBTokenBalances(fx.PreTokenBalances),
		PostTokenBalances: buildPBTokenBalances(fx.PostTokenBalances),
		LogMessages:       append([]string(nil), fx.Logs...),
	}

	transaction := &pb.Transaction{
		Signatures: [][]byte{signature},
		Message: &pb.Message{
			AccountKeys: accountKeys,
			Instructions: []*pb.CompiledInstruction{
				{
					ProgramIdIndex: uint32(programIndex),
					Accounts:       instrAccounts,
					Data:           []byte{0x01},
				},
			},
		},
	}

	return &pb.SubscribeUpdateTransaction{
		Transaction: &pb.SubscribeUpdateTransactionInfo{
			Signature:   signature,
			Transaction: transaction,
			Meta:        meta,
		},
		Slot: fx.Slot,
	}
}

func buildPBTokenBalances(entries []meteoraTokenBalance) []*pb.TokenBalance {
	balances := make([]*pb.TokenBalance, len(entries))
	for i, bal := range entries {
		balances[i] = &pb.TokenBalance{
			AccountIndex: bal.AccountIndex,
			Mint:         bal.Mint,
			Owner:        bal.Owner,
			UiTokenAmount: &pb.UiTokenAmount{
				Amount:   bal.Amount,
				Decimals: bal.Decimals,
			},
		}
	}
	return balances
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

func buildOrcaPoolData(t *testing.T, mintA, mintB, vaultA, vaultB []byte, feeRate uint16) []byte {
	t.Helper()
	const (
		orcaConfigOffset        = 8
		orcaFeeRateOffset       = orcaConfigOffset + 32 + 1 + 2 + 2
		orcaProtocolFeeOffset   = orcaFeeRateOffset + 2
		orcaLiquidityOffset     = orcaProtocolFeeOffset + 2
		orcaSqrtPriceOffset     = orcaLiquidityOffset + 16
		orcaTickOffset          = orcaSqrtPriceOffset + 16
		orcaProtocolFeeAOOffset = orcaTickOffset + 4
		orcaProtocolFeeBOOffset = orcaProtocolFeeAOOffset + 8
		orcaTokenMintAOffset    = orcaProtocolFeeBOOffset + 8
		orcaTokenVaultAOffset   = orcaTokenMintAOffset + 32
		orcaFeeGrowthAOffset    = orcaTokenVaultAOffset + 32
		orcaTokenMintBOffset    = orcaFeeGrowthAOffset + 16
		orcaTokenVaultBOffset   = orcaTokenMintBOffset + 32
		orcaRequiredLength      = orcaTokenVaultBOffset + 32
	)

	data := make([]byte, orcaRequiredLength)
	binary.LittleEndian.PutUint16(data[orcaFeeRateOffset:orcaFeeRateOffset+2], feeRate)

	copy(data[orcaTokenMintAOffset:orcaTokenMintAOffset+32], mintA)
	copy(data[orcaTokenVaultAOffset:orcaTokenVaultAOffset+32], vaultA)
	copy(data[orcaTokenMintBOffset:orcaTokenMintBOffset+32], mintB)
	copy(data[orcaTokenVaultBOffset:orcaTokenVaultBOffset+32], vaultB)

	return data
}

func generateAddress(seed byte) []byte {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = seed
	}
	return buf
}

func generateSignature(seed byte) []byte {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = seed
	}
	return buf
}

func accountDiscriminator(name string) []byte {
	sum := sha256.Sum256([]byte("account:" + name))
	return sum[:8]
}

func mustDecodeBase58(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base58.Decode(s)
	if err != nil {
		t.Fatalf("decode base58 %s: %v", s, err)
	}
	return b
}
