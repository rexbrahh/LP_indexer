package meteora

import (
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// Program IDs observed for Meteora pools. The map value captures the pool kind
// so the decoder can adjust logic when layouts diverge.
var programKinds = map[string]PoolKind{
	"cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG":  PoolKindCPMM, // Meteora DAMM v2 (CPMM)
	"LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo":  PoolKindCPMM, // Legacy Meteora pools
	"Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB": PoolKindDLMM, // Meteora DLMM
}

// ProgramKindForID reports the pool flavour associated with the provided
// program ID.
func ProgramKindForID(programID string) (PoolKind, bool) {
	kind, ok := programKinds[programID]
	return kind, ok
}

// SetProgramKind allows tests to extend or override program id lookups.
func SetProgramKind(programID string, kind PoolKind) {
	programKinds[programID] = kind
}

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
	Slot                uint64
	Signature           string
	Logs                []string
	Accounts            []string
	InstructionAccounts []byte
	PreTokenBalances    []*pb.TokenBalance
	PostTokenBalances   []*pb.TokenBalance
	ProgramID           string
	Kind                PoolKind
	Timestamp           time.Time
}
