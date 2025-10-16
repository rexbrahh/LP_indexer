package orca_whirlpool

import (
	"time"
)

const (
	// WhirlpoolProgramID is the Orca Whirlpools program address on Solana
	WhirlpoolProgramID = "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc"

	// SwapInstructionDiscriminator is the 8-byte discriminator for swap instructions
	SwapInstructionDiscriminator = uint64(0xf8c69e91e17587c8)
)

// SwapEvent represents a normalized swap event from Orca Whirlpools CLMM
type SwapEvent struct {
	// Transaction metadata
	Signature string    `json:"signature"`
	Slot      uint64    `json:"slot"`
	Timestamp time.Time `json:"timestamp"`

	// Pool identifiers
	PoolAddress    string `json:"pool_address"`
	MintA          string `json:"mint_a"`
	MintB          string `json:"mint_b"`

	// Swap direction and amounts
	AToB               bool   `json:"a_to_b"`
	AmountIn           uint64 `json:"amount_in"`
	AmountOut          uint64 `json:"amount_out"`

	// CLMM-specific: sqrt price tracking (Q64.64 fixed-point)
	SqrtPriceQ64Pre  string `json:"sqrt_price_q64_pre"`
	SqrtPriceQ64Post string `json:"sqrt_price_q64_post"`

	// Tick and liquidity state
	TickIndexPre   int32  `json:"tick_index_pre"`
	TickIndexPost  int32  `json:"tick_index_post"`
	LiquidityPre   string `json:"liquidity_pre"`
	LiquidityPost  string `json:"liquidity_post"`

	// Fee tracking
	FeeAmount      uint64 `json:"fee_amount"`
	ProtocolFee    uint64 `json:"protocol_fee"`

	// Normalized price and volume (computed from sqrt prices and decimals)
	Price          float64 `json:"price"`
	VolumeBase     float64 `json:"volume_base"`
	VolumeQuote    float64 `json:"volume_quote"`

	// Canonical ordering
	BaseAsset      string  `json:"base_asset"`
	QuoteAsset     string  `json:"quote_asset"`
}

// SwapInstruction represents the decoded swap instruction data
type SwapInstruction struct {
	Amount                   uint64 `json:"amount"`
	OtherAmountThreshold     uint64 `json:"other_amount_threshold"`
	SqrtPriceLimit           string `json:"sqrt_price_limit"` // u128 as string
	AmountSpecifiedIsInput   bool   `json:"amount_specified_is_input"`
	AToB                     bool   `json:"a_to_b"`
}

// WhirlpoolState represents the state of a Whirlpool account
type WhirlpoolState struct {
	WhirlpoolAddress  string `json:"whirlpool_address"`
	TokenMintA        string `json:"token_mint_a"`
	TokenMintB        string `json:"token_mint_b"`
	TokenVaultA       string `json:"token_vault_a"`
	TokenVaultB       string `json:"token_vault_b"`
	SqrtPrice         string `json:"sqrt_price"`         // u128 as string
	TickCurrentIndex  int32  `json:"tick_current_index"`
	Liquidity         string `json:"liquidity"`          // u128 as string
	FeeRate           uint16 `json:"fee_rate"`
	ProtocolFeeRate   uint16 `json:"protocol_fee_rate"`
}

// PostSwapUpdate captures the state changes after a swap
type PostSwapUpdate struct {
	AmountA            uint64 `json:"amount_a"`
	AmountB            uint64 `json:"amount_b"`
	NextLiquidity      string `json:"next_liquidity"`       // u128 as string
	NextTickIndex      int32  `json:"next_tick_index"`
	NextSqrtPrice      string `json:"next_sqrt_price"`      // u128 as string
	NextFeeGrowthGlobal string `json:"next_fee_growth_global"` // u128 as string
	NextProtocolFee    uint64 `json:"next_protocol_fee"`
}

// TickArray represents a sequence of tick states
type TickArray struct {
	StartTickIndex int32      `json:"start_tick_index"`
	Ticks          []TickData `json:"ticks"`
}

// TickData represents liquidity and fee data at a specific tick
type TickData struct {
	Initialized         bool   `json:"initialized"`
	LiquidityNet        string `json:"liquidity_net"`        // i128 as string
	LiquidityGross      string `json:"liquidity_gross"`      // u128 as string
	FeeGrowthOutsideA   string `json:"fee_growth_outside_a"` // u128 as string
	FeeGrowthOutsideB   string `json:"fee_growth_outside_b"` // u128 as string
}
