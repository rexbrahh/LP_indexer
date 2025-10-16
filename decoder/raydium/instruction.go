package raydium

import (
	"encoding/binary"
	"fmt"
)

// Raydium CLMM program ID
const ProgramID = "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK"

// SwapInstruction represents the layout of a Raydium CLMM swap instruction
// The instruction layout follows the on-chain program structure
type SwapInstruction struct {
	// Discriminator for the swap instruction (first 8 bytes)
	Discriminator [8]byte

	// Amount to swap (u64)
	Amount uint64

	// Other amount threshold (u64) - minimum amount out or maximum amount in
	OtherAmountThreshold uint64

	// Sqrt price limit (u128 as two u64s for Q64.64 fixed-point)
	SqrtPriceLimitX64Low  uint64
	SqrtPriceLimitX64High uint64

	// Is base input flag (bool)
	IsBaseInput bool
}

// SwapEvent represents a parsed swap event with canonical fields
type SwapEvent struct {
	// Pool/AMM identifier
	PoolAddress string

	// Token mints
	MintA string
	MintB string

	// Decimals for proper conversion
	DecimalsA uint8
	DecimalsB uint8

	// Swap amounts (raw, not scaled by decimals)
	AmountIn  uint64
	AmountOut uint64

	// Fee in basis points (1 bps = 0.01%)
	FeeBps uint16

	// Price as Q64.64 fixed-point
	SqrtPriceX64Low  uint64
	SqrtPriceX64High uint64

	// Direction: true if swapping A->B, false if B->A
	IsBaseInput bool

	// Transaction context
	Slot      uint64
	Signature string
	Timestamp int64
}

// ParseSwapInstruction parses raw instruction data into a SwapInstruction
func ParseSwapInstruction(data []byte) (*SwapInstruction, error) {
	// Minimum size check: 8 (discriminator) + 8 (amount) + 8 (threshold) + 16 (sqrt_price) + 1 (flag)
	if len(data) < 41 {
		return nil, fmt.Errorf("instruction data too short: got %d bytes, need at least 41", len(data))
	}

	instr := &SwapInstruction{}

	// Copy discriminator
	copy(instr.Discriminator[:], data[0:8])

	// Parse amounts and price limit
	instr.Amount = binary.LittleEndian.Uint64(data[8:16])
	instr.OtherAmountThreshold = binary.LittleEndian.Uint64(data[16:24])
	instr.SqrtPriceLimitX64Low = binary.LittleEndian.Uint64(data[24:32])
	instr.SqrtPriceLimitX64High = binary.LittleEndian.Uint64(data[32:40])

	// Parse direction flag
	instr.IsBaseInput = data[40] != 0

	return instr, nil
}

// SqrtPriceX64 returns the full 128-bit sqrt price as a Q64.64 fixed-point value
// This is stored as two u64s (low and high)
func (s *SwapInstruction) SqrtPriceX64() (low, high uint64) {
	return s.SqrtPriceLimitX64Low, s.SqrtPriceLimitX64High
}
