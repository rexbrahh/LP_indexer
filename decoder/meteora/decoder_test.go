package meteora

import (
	"testing"
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func TestDecodeSwapEvent_BaseSold(t *testing.T) {
	const (
		inputIdx  = 2
		outputIdx = 3
	)

	accounts := []string{
		"authority",
		"pool",
		"input_token_account",
		"output_token_account",
		"vault_a",
		"vault_b",
		"mint_base",
		"mint_quote",
		"payer",
		"token_program_a",
		"token_program_b",
	}

	ctx := &InstructionContext{
		Slot:      123,
		Signature: "sig",
		Accounts:  accounts,
		InstructionAccounts: []byte{
			0, 1, inputIdx, outputIdx, 4, 5, 6, 7, 8, 9, 10,
		},
		ProgramID: "cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG",
		Kind:      PoolKindCPMM,
		Timestamp: time.Unix(1_700_000_000, 0).UTC(),
		Logs: []string{
			"Program log: cpmm_reserves base=600000000 quote=2000000",
			"Program log: fee_bps=25",
		},
		PreTokenBalances: []*pb.TokenBalance{
			tokenBalance(inputIdx, "So11111111111111111111111111111111111111112", "1000000000", 9),
		},
		PostTokenBalances: []*pb.TokenBalance{
			tokenBalance(inputIdx, "So11111111111111111111111111111111111111112", "500000000", 9),
			tokenBalance(outputIdx, "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "1500000", 6),
		},
	}

	event, err := DecodeSwapEvent(nil, ctx)
	if err != nil {
		t.Fatalf("DecodeSwapEvent returned error: %v", err)
	}
	if event == nil {
		t.Fatal("expected swap event, got nil")
	}

	if event.BaseMint != "So11111111111111111111111111111111111111112" || event.QuoteMint != "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		t.Fatalf("unexpected canonical mints %s/%s", event.BaseMint, event.QuoteMint)
	}
	if event.BaseAmount != 500000000 {
		t.Fatalf("expected base amount 500000000, got %d", event.BaseAmount)
	}
	if event.QuoteAmount != 1500000 {
		t.Fatalf("expected quote amount 1500000, got %d", event.QuoteAmount)
	}
	if event.BaseDecreased {
		t.Fatal("expected base reserve to increase for base->quote swap")
	}
	if event.BaseDec != 9 || event.QuoteDec != 6 {
		t.Fatalf("unexpected decimals base=%d quote=%d", event.BaseDec, event.QuoteDec)
	}
	if event.RealReservesBase != 600000000 || event.RealReservesQuote != 2000000 {
		t.Fatalf("unexpected real reserves base=%d quote=%d", event.RealReservesBase, event.RealReservesQuote)
	}
	if event.FeeBps != 25 {
		t.Fatalf("unexpected fee bps %d", event.FeeBps)
	}
}

func TestDecodeSwapEvent_BaseBought(t *testing.T) {
	const (
		inputIdx  = 2
		outputIdx = 3
	)

	ctx := &InstructionContext{
		Slot:      456,
		Signature: "sig2",
		Accounts: []string{
			"authority",
			"pool",
			"input_token_account",
			"output_token_account",
			"vault_a",
			"vault_b",
			"mint_quote",
			"mint_base",
			"payer",
			"token_program_a",
			"token_program_b",
		},
		InstructionAccounts: []byte{0, 1, inputIdx, outputIdx, 4, 5, 6, 7, 8, 9, 10},
		ProgramID:           "Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB",
		Kind:                PoolKindDLMM,
		Timestamp:           time.Unix(1_700_100_000, 0).UTC(),
		Logs: []string{
			"Program log: virtual_reserves base=2500000 quote=780000000",
			"Program log: fee_bps=30",
		},
		PreTokenBalances: []*pb.TokenBalance{
			tokenBalance(inputIdx, "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "2000000", 6),
		},
		PostTokenBalances: []*pb.TokenBalance{
			tokenBalance(inputIdx, "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "500000", 6),
			tokenBalance(outputIdx, "So11111111111111111111111111111111111111112", "100000000", 9),
		},
	}

	event, err := DecodeSwapEvent(nil, ctx)
	if err != nil {
		t.Fatalf("DecodeSwapEvent returned error: %v", err)
	}
	if event == nil {
		t.Fatal("expected swap event, got nil")
	}

	if !event.BaseDecreased {
		t.Fatal("expected base reserve to decrease when quote->base")
	}
	if event.BaseAmount != 100000000 {
		t.Fatalf("unexpected base amount %d", event.BaseAmount)
	}
	if event.QuoteAmount != 1500000 {
		t.Fatalf("unexpected quote amount %d", event.QuoteAmount)
	}
	if event.DecBase != 9 || event.DecQuote != 6 {
		t.Fatalf("unexpected canonical decimals base=%d quote=%d", event.DecBase, event.DecQuote)
	}
	if event.VirtualReservesBase != 2500000 || event.VirtualReservesQuote != 780000000 {
		t.Fatalf("unexpected virtual reserves base=%d quote=%d", event.VirtualReservesBase, event.VirtualReservesQuote)
	}
	if event.FeeBps != 30 {
		t.Fatalf("unexpected fee bps %d", event.FeeBps)
	}
}

func tokenBalance(index uint32, mint string, amount string, decimals uint32) *pb.TokenBalance {
	return &pb.TokenBalance{
		AccountIndex: index,
		Mint:         mint,
		UiTokenAmount: &pb.UiTokenAmount{
			Amount:   amount,
			Decimals: decimals,
		},
	}
}
