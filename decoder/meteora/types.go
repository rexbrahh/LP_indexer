package meteora

import "time"

// MeteoraProgramID is the canonical program address for Meteora pools on
// Solana. Meteora deploys distinct programs for DLMM and CPMM pools; both
// variants funnel into this decoder.
const MeteoraProgramID = "METoRa111111111111111111111111111111111111111"

// PoolKind distinguishes between Meteora pool flavours.
type PoolKind string

const (
	// PoolKindDLMM represents Meteora's dynamic liquidity market maker pools.
	PoolKindDLMM PoolKind = "dlmm"
	// PoolKindCPMM represents Meteora's constant product pools.
	PoolKindCPMM PoolKind = "cpmm"
)

// SwapEvent captures the canonical fields extracted from a Meteora swap.
type SwapEvent struct {
	Signature string
	Slot      uint64
	Timestamp time.Time

	ProgramID string
	Pool      string
	Kind      PoolKind

	BaseMint    string
	QuoteMint   string
	BaseDec     uint8
	QuoteDec    uint8
	BaseAmount  uint64
	QuoteAmount uint64

	FeeBps uint32

	// DLMM specific fields populated when available.
	VirtualReservesBase  uint64
	VirtualReservesQuote uint64

	// CPMM specific reserves when emitted in logs.
	RealReservesBase  uint64
	RealReservesQuote uint64

	// Canonical ordering metadata determined by normalisation rules.
	MintBase  string
	MintQuote string
	DecBase   uint32
	DecQuote  uint32

	// Indicates whether the swap decreased the base reserve (base sold).
	BaseDecreased bool
}

// InstructionContext contains the ambient accounts and log data needed to
// interpret a Meteora swap instruction. The actual decoding logic will populate
// these fields using upstream ingestion data.
type InstructionContext struct {
	Slot      uint64
	Signature string
	Logs      []string
	Accounts  []string
	Timestamp time.Time
}
