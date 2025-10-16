# Orca Whirlpools CLMM Decoder Implementation

## Overview
This decoder handles Orca Whirlpools concentrated liquidity market maker (CLMM) swap events on Solana.

## Features Implemented

### 1. Core Types (`types.go`)
- `SwapEvent`: Complete swap event structure with sqrt price pre/post, tick indices, liquidity snapshots
- `SwapInstruction`: Decoded instruction data
- `WhirlpoolState`: Pool state representation
- `PostSwapUpdate`: State changes after swap
- Program constants (Program ID, instruction discriminator)

### 2. Fixed-Point Math (`decoder/common/fixed_point.go`)
- Q64.64 fixed-point price representation
- `SqrtPriceQ64ToFloat`: Converts Q64.64 sqrt price to float64 price
- `FloatToSqrtPriceQ64`: Converts float64 price to Q64.64 sqrt price
- `TickIndexToSqrtPrice`: Tick index to sqrt price conversion
- `SqrtPriceToTickIndex`: Sqrt price to tick index conversion
- `ScaleAmount`: Scales raw token amounts by decimals
- `UnscaleAmount`: Converts scaled amounts back to raw units
- `CalculatePriceFromAmounts`: Calculates effective price from swap amounts

### 3. Mint Metadata Interface (`decoder/common/mint_metadata.go`)
- `MintMetadataProvider`: Interface for Engineer B to implement
- `InMemoryMintMetadataProvider`: Stub implementation for testing
- `DetermineBaseQuote`: Canonical base/quote ordering (USDC > USDT > SOL)
- Pre-populated with common Solana tokens (SOL, USDC, USDT, ORCA)

### 4. Decoder Logic (`decoder.go`)
- `DecodeSwapTransaction`: Main decoder function
- Instruction data parsing (discriminator, amounts, sqrt prices)
- Balance change calculation
- Fee calculation (swap fee + protocol fee)
- Canonical ordering enforcement
- Volume scaling and price normalization

### 5. Test Fixtures (`fixtures_test.go`)
- SOL/USDC swap fixture
- USDC/USDT swap fixture
- Validates canonical ordering
- Tests volume scaling

### 6. Comprehensive Tests (`decoder_test.go`, `fixed_point_test.go`, `mint_metadata_test.go`)
- Swap transaction decoding
- Canonical base/quote ordering
- Volume scaling validation
- Fee calculation validation
- Fixed-point math conversions
- Mint metadata lookups

## Architecture Decisions

### Q64.64 Fixed-Point Representation
Orca Whirlpools uses Q64.64 format for sqrt prices:
- 128-bit number with 64 bits for integer, 64 bits for fractional
- Stored as big.Int to handle large numbers
- Price = (sqrt_price / 2^64)^2

### Canonical Ordering
Priority: USDC > USDT > SOL > others
- Ensures consistent base/quote pairing across the system
- Simplifies price aggregation and comparison

### Fee Calculation
- Swap fee: (amountIn * feeRate) / 10000
- Protocol fee: (swapFee * protocolFeeRate) / 10000
- Fee rates in basis points (e.g., 30 = 0.3%)

## Coordination with Engineer B

The `MintMetadataProvider` interface is ready for Engineer B to implement:

```go
type MintMetadataProvider interface {
    GetMintMetadata(mintAddress string) (*MintMetadata, error)
    GetDecimals(mintAddress string) (uint8, error)
    CacheMintMetadata(mintAddresses []string) error
}
```

Engineer B should:
1. Implement this interface with a real data source (RPC, cache, database)
2. Populate decimal data for all relevant Solana tokens
3. Handle cache invalidation and updates
4. Ensure thread-safe access

## Test Results

All tests passing:
- ✅ Fixed-point math conversions
- ✅ Mint metadata lookups
- ✅ Canonical ordering
- ✅ Volume scaling
- ✅ Fee calculations
- ✅ Swap transaction decoding

## Usage Example

```go
// Create metadata provider
metadataProvider := common.NewInMemoryMintMetadataProvider()

// Create decoder
decoder := orca_whirlpool.NewDecoder(metadataProvider)

// Decode swap transaction
event, err := decoder.DecodeSwapTransaction(
    signature,
    slot,
    timestamp,
    instructionData,
    accounts,
    preBalances,
    postBalances,
    poolStatePre,
    poolStatePost,
)
```

## Next Steps

1. Integrate with ingestor pipeline
2. Replace `InMemoryMintMetadataProvider` with Engineer B's implementation
3. Add support for other Whirlpool instructions (position open/close, liquidity add/remove)
4. Performance testing with production data
5. Add metrics and monitoring
