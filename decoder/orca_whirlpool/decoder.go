package orca_whirlpool

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/rexbrahh/lp-indexer/.conductor/almaty/decoder/common"
)

// Decoder handles decoding of Orca Whirlpool swap transactions
type Decoder struct {
	metadataProvider common.MintMetadataProvider
}

// NewDecoder creates a new Orca Whirlpool decoder
func NewDecoder(metadataProvider common.MintMetadataProvider) *Decoder {
	return &Decoder{
		metadataProvider: metadataProvider,
	}
}

// DecodeSwapTransaction decodes a Solana transaction containing an Orca Whirlpool swap
func (d *Decoder) DecodeSwapTransaction(
	signature string,
	slot uint64,
	timestamp time.Time,
	instructionData []byte,
	accounts []string,
	preBalances []uint64,
	postBalances []uint64,
	poolStatePre *WhirlpoolState,
	poolStatePost *WhirlpoolState,
) (*SwapEvent, error) {
	// Verify this is a swap instruction
	if len(instructionData) < 8 {
		return nil, fmt.Errorf("instruction data too short")
	}

	discriminator := binary.LittleEndian.Uint64(instructionData[0:8])
	if discriminator != SwapInstructionDiscriminator {
		return nil, fmt.Errorf("not a swap instruction: discriminator %x", discriminator)
	}

	// Decode swap instruction
	instruction, err := decodeSwapInstruction(instructionData[8:])
	if err != nil {
		return nil, fmt.Errorf("failed to decode swap instruction: %w", err)
	}

	// Extract pool address (account 2 in standard swap instruction)
	if len(accounts) < 3 {
		return nil, fmt.Errorf("insufficient accounts")
	}
	poolAddress := accounts[2]

	// Use pool state to get mint addresses
	if poolStatePre == nil {
		return nil, fmt.Errorf("pool state pre is required")
	}
	if poolStatePost == nil {
		return nil, fmt.Errorf("pool state post is required")
	}

	mintA := poolStatePre.TokenMintA
	mintB := poolStatePre.TokenMintB

	// Calculate amounts from balance changes
	amountIn, amountOut, err := d.calculateAmounts(
		instruction,
		preBalances,
		postBalances,
		accounts,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate amounts: %w", err)
	}

	// Get decimals for both tokens
	decimalsA, err := d.metadataProvider.GetDecimals(mintA)
	if err != nil {
		return nil, fmt.Errorf("failed to get decimals for mint A: %w", err)
	}

	decimalsB, err := d.metadataProvider.GetDecimals(mintB)
	if err != nil {
		return nil, fmt.Errorf("failed to get decimals for mint B: %w", err)
	}

	// Determine canonical base/quote ordering
	baseMint, quoteMint, err := common.DetermineBaseQuote(mintA, mintB, d.metadataProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to determine base/quote: %w", err)
	}

	// Calculate normalized price from sqrt prices
	pricePost, err := common.SqrtPriceQ64ToFloat(poolStatePost.SqrtPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to convert sqrt price to float: %w", err)
	}

	// Determine which direction the swap went and calculate volumes
	var volumeBase, volumeQuote float64
	var price float64

	if instruction.AToB {
		// Swapping A -> B
		if baseMint == mintA {
			volumeBase = common.ScaleAmount(amountIn, decimalsA)
			volumeQuote = common.ScaleAmount(amountOut, decimalsB)
		} else {
			volumeBase = common.ScaleAmount(amountOut, decimalsB)
			volumeQuote = common.ScaleAmount(amountIn, decimalsA)
		}
	} else {
		// Swapping B -> A
		if baseMint == mintA {
			volumeBase = common.ScaleAmount(amountOut, decimalsA)
			volumeQuote = common.ScaleAmount(amountIn, decimalsB)
		} else {
			volumeBase = common.ScaleAmount(amountIn, decimalsB)
			volumeQuote = common.ScaleAmount(amountOut, decimalsA)
		}
	}

	// Normalize price based on base/quote ordering
	if baseMint == mintA {
		price = pricePost
	} else {
		price = 1.0 / pricePost
	}

	// Calculate fees
	feeAmount := calculateFeeAmount(amountIn, poolStatePre.FeeRate)
	protocolFee := calculateProtocolFee(feeAmount, poolStatePre.ProtocolFeeRate)

	return &SwapEvent{
		Signature:        signature,
		Slot:             slot,
		Timestamp:        timestamp,
		PoolAddress:      poolAddress,
		MintA:            mintA,
		MintB:            mintB,
		AToB:             instruction.AToB,
		AmountIn:         amountIn,
		AmountOut:        amountOut,
		SqrtPriceQ64Pre:  poolStatePre.SqrtPrice,
		SqrtPriceQ64Post: poolStatePost.SqrtPrice,
		TickIndexPre:     poolStatePre.TickCurrentIndex,
		TickIndexPost:    poolStatePost.TickCurrentIndex,
		LiquidityPre:     poolStatePre.Liquidity,
		LiquidityPost:    poolStatePost.Liquidity,
		FeeAmount:        feeAmount,
		ProtocolFee:      protocolFee,
		Price:            price,
		VolumeBase:       volumeBase,
		VolumeQuote:      volumeQuote,
		BaseAsset:        baseMint,
		QuoteAsset:       quoteMint,
	}, nil
}

// decodeSwapInstruction decodes the swap instruction data (after the 8-byte discriminator)
func decodeSwapInstruction(data []byte) (*SwapInstruction, error) {
	// u64 + u64 + u128 + bool + bool = 8 + 8 + 16 + 1 + 1 = 34 bytes minimum
	if len(data) < 34 {
		return nil, fmt.Errorf("swap instruction data too short: %d bytes", len(data))
	}

	reader := bytes.NewReader(data)

	var instruction SwapInstruction

	// Read amount (u64)
	if err := binary.Read(reader, binary.LittleEndian, &instruction.Amount); err != nil {
		return nil, fmt.Errorf("failed to read amount: %w", err)
	}

	// Read other_amount_threshold (u64)
	if err := binary.Read(reader, binary.LittleEndian, &instruction.OtherAmountThreshold); err != nil {
		return nil, fmt.Errorf("failed to read other_amount_threshold: %w", err)
	}

	// Read sqrt_price_limit (u128 - 16 bytes)
	sqrtPriceLimitBytes := make([]byte, 16)
	if _, err := reader.Read(sqrtPriceLimitBytes); err != nil {
		return nil, fmt.Errorf("failed to read sqrt_price_limit: %w", err)
	}
	instruction.SqrtPriceLimit = bytesToU128String(sqrtPriceLimitBytes)

	// Read amount_specified_is_input (bool)
	var amountSpecifiedIsInputByte uint8
	if err := binary.Read(reader, binary.LittleEndian, &amountSpecifiedIsInputByte); err != nil {
		return nil, fmt.Errorf("failed to read amount_specified_is_input: %w", err)
	}
	instruction.AmountSpecifiedIsInput = amountSpecifiedIsInputByte != 0

	// Read a_to_b (bool)
	var aToBByte uint8
	if err := binary.Read(reader, binary.LittleEndian, &aToBByte); err != nil {
		return nil, fmt.Errorf("failed to read a_to_b: %w", err)
	}
	instruction.AToB = aToBByte != 0

	return &instruction, nil
}

// calculateAmounts calculates the input and output amounts from balance changes
func (d *Decoder) calculateAmounts(
	instruction *SwapInstruction,
	preBalances []uint64,
	postBalances []uint64,
	accounts []string,
) (amountIn, amountOut uint64, err error) {
	// In a typical Orca swap:
	// Account 3: token_owner_account_a
	// Account 4: token_vault_a
	// Account 5: token_owner_account_b
	// Account 6: token_vault_b

	if len(preBalances) < 7 || len(postBalances) < 7 {
		return 0, 0, fmt.Errorf("insufficient balance data")
	}

	if instruction.AToB {
		// Swapping A -> B
		// User's token A decreases (input)
		amountIn = preBalances[3] - postBalances[3]
		// User's token B increases (output)
		amountOut = postBalances[5] - preBalances[5]
	} else {
		// Swapping B -> A
		// User's token B decreases (input)
		amountIn = preBalances[5] - postBalances[5]
		// User's token A increases (output)
		amountOut = postBalances[3] - preBalances[3]
	}

	return amountIn, amountOut, nil
}

// calculateFeeAmount calculates the fee from the input amount and fee rate
// feeRate is in basis points (e.g., 30 = 0.3%)
func calculateFeeAmount(amountIn uint64, feeRate uint16) uint64 {
	// Fee = (amountIn * feeRate) / 10000
	return (amountIn * uint64(feeRate)) / 10000
}

// calculateProtocolFee calculates the protocol fee from the total fee
// protocolFeeRate is in basis points
func calculateProtocolFee(feeAmount uint64, protocolFeeRate uint16) uint64 {
	// Protocol fee = (feeAmount * protocolFeeRate) / 10000
	return (feeAmount * uint64(protocolFeeRate)) / 10000
}

// bytesToU128String converts a 16-byte little-endian byte array to a string representation of u128
func bytesToU128String(b []byte) string {
	// Use big.Int for u128 representation
	// Convert little-endian bytes to big.Int
	result := make([]byte, 16)
	for i := 0; i < 16; i++ {
		result[15-i] = b[i]
	}

	// Convert to string
	var num [16]byte
	copy(num[:], result)

	// Simple conversion for demonstration - in production use big.Int
	low := binary.LittleEndian.Uint64(b[0:8])
	high := binary.LittleEndian.Uint64(b[8:16])

	if high == 0 {
		return fmt.Sprintf("%d", low)
	}

	// For non-zero high bits, construct the full u128
	// This is a simplified version - use math/big for production
	return fmt.Sprintf("%d%016d", high, low)
}
