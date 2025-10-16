package raydium

import (
	"fmt"

	"github.com/rexbrahh/lp-indexer/decoder/common"
)

// AccountKeys represents the accounts involved in a swap transaction
type AccountKeys struct {
	PoolAddress    string
	TokenVaultA    string
	TokenVaultB    string
	MintA          string
	MintB          string
	UserTokenA     string
	UserTokenB     string
	TickArrayLower string
	TickArrayUpper string
	ObservationKey string
}

// SwapContext provides additional context needed to parse a swap event
type SwapContext struct {
	Accounts   AccountKeys
	PreTokenA  uint64 // Pre-swap balance of token A vault
	PostTokenA uint64 // Post-swap balance of token A vault
	PreTokenB  uint64 // Pre-swap balance of token B vault
	PostTokenB uint64 // Post-swap balance of token B vault
	DecimalsA  uint8
	DecimalsB  uint8
	FeeBps     uint16
	Slot       uint64
	Signature  string
	Timestamp  int64
}

// ParseSwapEvent parses a swap instruction and context into a canonical SwapEvent
func ParseSwapEvent(instr *SwapInstruction, ctx *SwapContext) (*SwapEvent, error) {
	if instr == nil {
		return nil, fmt.Errorf("instruction cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	event := &SwapEvent{
		PoolAddress:      ctx.Accounts.PoolAddress,
		MintA:            ctx.Accounts.MintA,
		MintB:            ctx.Accounts.MintB,
		DecimalsA:        ctx.DecimalsA,
		DecimalsB:        ctx.DecimalsB,
		FeeBps:           ctx.FeeBps,
		SqrtPriceX64Low:  instr.SqrtPriceLimitX64Low,
		SqrtPriceX64High: instr.SqrtPriceLimitX64High,
		IsBaseInput:      instr.IsBaseInput,
		Slot:             ctx.Slot,
		Signature:        ctx.Signature,
		Timestamp:        ctx.Timestamp,
	}

	deltaA := int64(ctx.PostTokenA) - int64(ctx.PreTokenA)
	deltaB := int64(ctx.PostTokenB) - int64(ctx.PreTokenB)

	switch {
	case deltaA > 0 && deltaB < 0:
		event.AmountIn = uint64(deltaA)
		event.AmountOut = uint64(-deltaB)
		event.IsBaseInput = true
	case deltaA < 0 && deltaB > 0:
		event.AmountIn = uint64(deltaB)
		event.AmountOut = uint64(-deltaA)
		event.IsBaseInput = false
	default:
		return nil, fmt.Errorf("unable to determine swap direction: deltaA=%d deltaB=%d", deltaA, deltaB)
	}

	// Validate amounts
	if event.AmountIn == 0 {
		return nil, fmt.Errorf("invalid swap: amount in is zero")
	}
	if event.AmountOut == 0 {
		return nil, fmt.Errorf("invalid swap: amount out is zero")
	}

	return event, nil
}

// NormalizeToCanonicalPair applies canonical pair resolution to a swap event
// This ensures consistent ordering (e.g., SOL/USDC not USDC/SOL) and adjusts
// amounts and direction flags accordingly
func (e *SwapEvent) NormalizeToCanonicalPair() (*common.CanonicalPair, error) {
	pair, err := common.ResolvePair(e.MintA, e.MintB)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve canonical pair: %w", err)
	}

	// If the pair is inverted, we need to swap our perspective
	if pair.Inverted {
		// Swap the mints and decimals to match canonical order
		e.MintA, e.MintB = e.MintB, e.MintA
		e.DecimalsA, e.DecimalsB = e.DecimalsB, e.DecimalsA

		// Invert the direction flag
		e.IsBaseInput = !e.IsBaseInput
	}

	return pair, nil
}

// CalculatePrice computes the spot price from Q64.64 sqrt price
// Price = (sqrtPrice / 2^64)^2
// Returns the price as a float64 for convenience
func (e *SwapEvent) CalculatePrice() float64 {
	baseAmount := common.ScaleAmount(e.AmountIn, e.DecimalsA)
	quoteAmount := common.ScaleAmount(e.AmountOut, e.DecimalsB)

	if !e.IsBaseInput {
		baseAmount = common.ScaleAmount(e.AmountOut, e.DecimalsA)
		quoteAmount = common.ScaleAmount(e.AmountIn, e.DecimalsB)
	}

	if baseAmount == 0 {
		return 0
	}

	return quoteAmount / baseAmount
}

// CalculateVolume returns the swap volume in terms of the quote token
// For A->B swaps, volume is AmountOut (in token B)
// For B->A swaps, volume is AmountIn (in token B)
func (e *SwapEvent) CalculateVolume() uint64 {
	if e.IsBaseInput {
		// A->B: volume is the amount of B received
		return e.AmountOut
	}
	// B->A: volume is the amount of B swapped
	return e.AmountIn
}
