package orca_whirlpool

import (
	"math"
	"testing"

	"github.com/rexbrahh/lp-indexer/decoder/common"
)

func TestDecoder_DecodeSwapTransaction(t *testing.T) {
	// Setup metadata provider with test mints
	metadataProvider := common.NewInMemoryMintMetadataProvider()

	decoder := NewDecoder(metadataProvider)

	fixtures := GetTestFixtures()

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			event, err := decoder.DecodeSwapTransaction(
				fixture.Signature,
				fixture.Slot,
				fixture.Timestamp,
				fixture.InstructionData,
				fixture.Accounts,
				fixture.PreBalances,
				fixture.PostBalances,
				fixture.PoolStatePre,
				fixture.PoolStatePost,
			)

			if err != nil {
				t.Fatalf("DecodeSwapTransaction failed: %v", err)
			}

			// Validate basic fields
			if event.Signature != fixture.ExpectedEvent.Signature {
				t.Errorf("Signature mismatch: got %s, want %s", event.Signature, fixture.ExpectedEvent.Signature)
			}

			if event.Slot != fixture.ExpectedEvent.Slot {
				t.Errorf("Slot mismatch: got %d, want %d", event.Slot, fixture.ExpectedEvent.Slot)
			}

			if event.PoolAddress != fixture.ExpectedEvent.PoolAddress {
				t.Errorf("PoolAddress mismatch: got %s, want %s", event.PoolAddress, fixture.ExpectedEvent.PoolAddress)
			}

			// Validate mints
			if event.MintA != fixture.ExpectedEvent.MintA {
				t.Errorf("MintA mismatch: got %s, want %s", event.MintA, fixture.ExpectedEvent.MintA)
			}

			if event.MintB != fixture.ExpectedEvent.MintB {
				t.Errorf("MintB mismatch: got %s, want %s", event.MintB, fixture.ExpectedEvent.MintB)
			}

			// Validate swap direction
			if event.AToB != fixture.ExpectedEvent.AToB {
				t.Errorf("AToB mismatch: got %v, want %v", event.AToB, fixture.ExpectedEvent.AToB)
			}

			// Validate amounts
			if event.AmountIn != fixture.ExpectedEvent.AmountIn {
				t.Errorf("AmountIn mismatch: got %d, want %d", event.AmountIn, fixture.ExpectedEvent.AmountIn)
			}

			if event.AmountOut != fixture.ExpectedEvent.AmountOut {
				t.Errorf("AmountOut mismatch: got %d, want %d", event.AmountOut, fixture.ExpectedEvent.AmountOut)
			}

			// Validate sqrt prices
			if event.SqrtPriceQ64Pre != fixture.ExpectedEvent.SqrtPriceQ64Pre {
				t.Errorf("SqrtPriceQ64Pre mismatch: got %s, want %s", event.SqrtPriceQ64Pre, fixture.ExpectedEvent.SqrtPriceQ64Pre)
			}

			if event.SqrtPriceQ64Post != fixture.ExpectedEvent.SqrtPriceQ64Post {
				t.Errorf("SqrtPriceQ64Post mismatch: got %s, want %s", event.SqrtPriceQ64Post, fixture.ExpectedEvent.SqrtPriceQ64Post)
			}

			// Validate tick indices
			if event.TickIndexPre != fixture.ExpectedEvent.TickIndexPre {
				t.Errorf("TickIndexPre mismatch: got %d, want %d", event.TickIndexPre, fixture.ExpectedEvent.TickIndexPre)
			}

			if event.TickIndexPost != fixture.ExpectedEvent.TickIndexPost {
				t.Errorf("TickIndexPost mismatch: got %d, want %d", event.TickIndexPost, fixture.ExpectedEvent.TickIndexPost)
			}

			// Validate liquidity
			if event.LiquidityPre != fixture.ExpectedEvent.LiquidityPre {
				t.Errorf("LiquidityPre mismatch: got %s, want %s", event.LiquidityPre, fixture.ExpectedEvent.LiquidityPre)
			}

			if event.LiquidityPost != fixture.ExpectedEvent.LiquidityPost {
				t.Errorf("LiquidityPost mismatch: got %s, want %s", event.LiquidityPost, fixture.ExpectedEvent.LiquidityPost)
			}

			// Validate fees
			if event.FeeAmount != fixture.ExpectedEvent.FeeAmount {
				t.Errorf("FeeAmount mismatch: got %d, want %d", event.FeeAmount, fixture.ExpectedEvent.FeeAmount)
			}

			if event.ProtocolFee != fixture.ExpectedEvent.ProtocolFee {
				t.Errorf("ProtocolFee mismatch: got %d, want %d", event.ProtocolFee, fixture.ExpectedEvent.ProtocolFee)
			}

			// Validate floats with tolerance
			if math.Abs(event.Price-fixture.ExpectedEvent.Price) > 1e-6 {
				t.Errorf("Price mismatch: got %f, want %f", event.Price, fixture.ExpectedEvent.Price)
			}
			if math.Abs(event.VolumeBase-fixture.ExpectedEvent.VolumeBase) > 1e-6 {
				t.Errorf("VolumeBase mismatch: got %f, want %f", event.VolumeBase, fixture.ExpectedEvent.VolumeBase)
			}
			if math.Abs(event.VolumeQuote-fixture.ExpectedEvent.VolumeQuote) > 1e-6 {
				t.Errorf("VolumeQuote mismatch: got %f, want %f", event.VolumeQuote, fixture.ExpectedEvent.VolumeQuote)
			}

			// Validate canonical ordering
			if event.BaseAsset != fixture.ExpectedEvent.BaseAsset {
				t.Errorf("BaseAsset mismatch: got %s, want %s", event.BaseAsset, fixture.ExpectedEvent.BaseAsset)
			}

			if event.QuoteAsset != fixture.ExpectedEvent.QuoteAsset {
				t.Errorf("QuoteAsset mismatch: got %s, want %s", event.QuoteAsset, fixture.ExpectedEvent.QuoteAsset)
			}
		})
	}
}

func TestDecoder_CanonicalBaseQuoteOrdering(t *testing.T) {
	metadataProvider := common.NewInMemoryMintMetadataProvider()

	tests := []struct {
		name          string
		mintA         string
		mintB         string
		expectedBase  string
		expectedQuote string
	}{
		{
			name:          "USDC_SOL_pair",
			mintA:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			mintB:         "So11111111111111111111111111111111111111112",  // SOL
			expectedBase:  "So11111111111111111111111111111111111111112",
			expectedQuote: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		{
			name:          "USDC_USDT_pair",
			mintA:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			mintB:         "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", // USDT
			expectedBase:  "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			expectedQuote: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		{
			name:          "USDT_SOL_pair",
			mintA:         "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", // USDT
			mintB:         "So11111111111111111111111111111111111111112",  // SOL
			expectedBase:  "So11111111111111111111111111111111111111112",
			expectedQuote: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, quote, err := common.DetermineBaseQuote(tt.mintA, tt.mintB, metadataProvider)
			if err != nil {
				t.Fatalf("DetermineBaseQuote failed: %v", err)
			}

			if base != tt.expectedBase {
				t.Errorf("Base mismatch: got %s, want %s", base, tt.expectedBase)
			}

			if quote != tt.expectedQuote {
				t.Errorf("Quote mismatch: got %s, want %s", quote, tt.expectedQuote)
			}
		})
	}
}

func TestDecoder_VolumeScaling(t *testing.T) {
	tests := []struct {
		name           string
		amount         uint64
		decimals       uint8
		expectedScaled float64
	}{
		{
			name:           "SOL_amount",
			amount:         1000000000, // 1 SOL
			decimals:       9,
			expectedScaled: 1.0,
		},
		{
			name:           "USDC_amount",
			amount:         1000000, // 1 USDC
			decimals:       6,
			expectedScaled: 1.0,
		},
		{
			name:           "USDC_large_amount",
			amount:         1234567890, // 1234.56789 USDC
			decimals:       6,
			expectedScaled: 1234.56789,
		},
		{
			name:           "SOL_fractional",
			amount:         123456789, // 0.123456789 SOL
			decimals:       9,
			expectedScaled: 0.123456789,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaled := common.ScaleAmount(tt.amount, tt.decimals)

			// Allow for small floating point errors
			if abs(scaled-tt.expectedScaled) > 0.0000001 {
				t.Errorf("ScaleAmount mismatch: got %f, want %f", scaled, tt.expectedScaled)
			}
		})
	}
}

func TestDecoder_FeeCalculation(t *testing.T) {
	tests := []struct {
		name                string
		amountIn            uint64
		feeRate             uint16
		expectedFeeAmount   uint64
		protocolFeeRate     uint16
		expectedProtocolFee uint64
	}{
		{
			name:                "0.3%_fee_1_SOL",
			amountIn:            1000000000, // 1 SOL
			feeRate:             30,         // 0.3%
			expectedFeeAmount:   3000000,    // 0.003 SOL
			protocolFeeRate:     200,        // 2% of fee
			expectedProtocolFee: 60000,      // 2% of 0.003
		},
		{
			name:                "0.1%_fee_1000_USDC",
			amountIn:            1000000000,
			feeRate:             10,
			expectedFeeAmount:   1000000,
			protocolFeeRate:     100,
			expectedProtocolFee: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feeAmount := calculateFeeAmount(tt.amountIn, tt.feeRate)
			if feeAmount != tt.expectedFeeAmount {
				t.Errorf("Fee amount mismatch: got %d, want %d", feeAmount, tt.expectedFeeAmount)
			}

			protocolFee := calculateProtocolFee(feeAmount, tt.protocolFeeRate)
			if protocolFee != tt.expectedProtocolFee {
				t.Errorf("Protocol fee mismatch: got %d, want %d", protocolFee, tt.expectedProtocolFee)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
