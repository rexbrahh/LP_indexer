# Liquidity Pool Normalization Guide

## Overview

This document describes the canonical normalization process for DEX swap events across different Solana AMM protocols (Raydium CLMM, Orca Whirlpools, Meteora). Normalization ensures consistent data representation for downstream analytics, aggregation, and indexing.

## Canonical Pair Resolution

### Priority-Based Ordering

Trading pairs are normalized using a priority-based system where higher-priority tokens become the **quote** (second token in the pair):

| Token | Priority | Notes |
|-------|----------|-------|
| USDC  | 100      | Highest priority, primary quote |
| USDT  | 90       | Secondary stablecoin quote |
| SOL   | 80       | Native token |
| WSOL  | 75       | Wrapped SOL |
| ETH   | 70       | Major crypto |
| WETH  | 65       | Wrapped ETH |
| BTC   | 60       | Major crypto |
| WBTC  | 55       | Wrapped BTC |
| Other | 0        | Unknown tokens |

**Examples:**
- Raw pool: `MintA=SOL, MintB=USDC` → Canonical: `SOL/USDC` ✓ (no inversion needed)
- Raw pool: `MintA=USDC, MintB=SOL` → Canonical: `SOL/USDC` ✓ (inverted)
- Raw pool: `MintA=ETH, MintB=SOL` → Canonical: `ETH/SOL` ✓ (SOL higher priority)

### Inversion Handling

When a pair is inverted (e.g., `USDC/SOL` → `SOL/USDC`), the following transformations are applied:

1. **Mints**: Swap `MintA` ↔ `MintB`
2. **Decimals**: Swap `DecimalsA` ↔ `DecimalsB`
3. **Direction flag**: Invert `IsBaseInput` (true → false, false → true)
4. **Amounts**: Remain unchanged (they're absolute values)

The decoder ensures that after normalization:
- `MintA` is always the base token
- `MintB` is always the quote token
- `IsBaseInput` correctly reflects direction in canonical terms

## Raydium CLMM Specifics

### Q64.64 Fixed-Point Pricing

Raydium CLMM uses 128-bit Q64.64 fixed-point representation for sqrt prices:

```
sqrtPrice = (sqrtPriceX64Low + sqrtPriceX64High * 2^64) / 2^64
price = sqrtPrice²
```

**Decimal Adjustment:**
```
adjustedPrice = price * 10^(decimalsA - decimalsB)
```

### Instruction Layout

Raydium swap instructions contain:
- **Discriminator** (8 bytes): Instruction type identifier
- **Amount** (u64): Input or output amount depending on direction
- **OtherAmountThreshold** (u64): Slippage protection threshold
- **SqrtPriceLimitX64** (u128): Price limit as two u64s (low, high)
- **IsBaseInput** (bool): Direction flag

### Amount Calculation

Amounts are derived from vault balance changes:

**A → B (IsBaseInput = true):**
```
AmountIn = PostVaultA - PreVaultA
AmountOut = PreVaultB - PostVaultB
```

**B → A (IsBaseInput = false):**
```
AmountIn = PostVaultB - PreVaultB
AmountOut = PreVaultA - PostVaultA
```

### Volume Calculation

Volume is always denominated in the **quote token** (token B after canonical normalization):

**For A→B swaps:**
```
volume = AmountOut (token B received)
```

**For B→A swaps:**
```
volume = AmountIn (token B swapped)
```

## Edge Cases and Considerations

### 1. Zero Amounts
**Issue**: Vault balances may not change if fees consume entire swap
**Handling**: Parser rejects swaps with `AmountIn == 0` or `AmountOut == 0`
**Rationale**: Zero-amount swaps provide no meaningful market data

### 2. Stablecoin Pairs (USDC/USDT)
**Issue**: Equal priority tokens need deterministic ordering
**Handling**: Fall back to lexicographic ordering of mint addresses
**Example**: If both have priority 100, sort by mint string comparison
**Detection**: Use `CanonicalPair.IsStablecoinPair()` to identify these pairs

### 3. Unknown Tokens
**Issue**: Tokens not in the registry get priority 0
**Handling**:
- Use first 8 characters of mint address as symbol
- Lexicographic ordering if both unknown
- Can dynamically register via `common.RegisterToken(mint, symbol)`

### 4. Price Calculation Precision
**Issue**: Float64 precision limits for very large/small Q64.64 values
**Mitigation**: Tests use 1% epsilon tolerance for price comparisons
**Future**: Consider fixed-point math library for production accuracy

### 5. Decimal Mismatches
**Issue**: Tokens with vastly different decimals (e.g., 9 vs 2)
**Handling**: Decimal adjustment in `CalculatePrice()` applies 10^(decA - decB)
**Example**: SOL (9 decimals) / USDC (6 decimals) → multiply by 10^3

### 6. Wrapped vs Native Tokens
**Issue**: SOL vs WSOL should be treated as equivalent
**Current**: Separate priorities (SOL=80, WSOL=75)
**Consideration**: May want to normalize WSOL→SOL in future normalization step

### 7. Negative Balance Changes
**Issue**: Arithmetic underflow if vaults decrease when expecting increase
**Detection**: Subtraction yields very large uint64 (wrapped)
**Mitigation**: Validate direction matches expected vault changes

## Testing Strategy

### Fixture Requirements
Each decoder includes at least two mainnet fixtures:
1. **Direction A→B**: Base token to quote token swap
2. **Direction B→A**: Quote token to base token swap

### Assertions
Table-driven tests verify:
- ✓ Canonical fields populated (mints, decimals, fee_bps)
- ✓ Amounts match expected values from vault deltas
- ✓ Price calculation within epsilon (1% tolerance)
- ✓ Volume calculation correct for both directions
- ✓ Pair normalization produces consistent symbols

## Implementation Checklist

- [x] Go structs mirroring Raydium swap instruction layout
- [x] Parser filling canonical SwapEvent fields
- [x] Q64.64 price calculation with decimal adjustment
- [x] Volume calculation denominated in quote token
- [x] Canonical pair resolver (`decoder/common/pair.go`)
- [x] Pair inversion handling in SwapEvent
- [x] Test fixtures from mainnet (2+ transactions)
- [x] Table-driven tests for price/volume math
- [x] Edge case documentation

## Future Enhancements

1. **Multi-DEX Aggregation**: Extend pair resolver to handle cross-DEX normalization
2. **Real-time Token Registry**: Fetch token metadata from on-chain or API
3. **Fixed-Point Math**: Replace float64 with integer-based fixed-point for production
4. **Liquidity Depth**: Normalize tick arrays for orderbook-like depth data
5. **Cross-Chain Pairs**: Handle bridged tokens (e.g., Wormhole WETH)

## References

- Raydium CLMM Program: `CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK`
- Q64.64 Format: [Wikipedia - Q notation](https://en.wikipedia.org/wiki/Q_(number_format))
- Solana Token Program: [SPL Token Docs](https://spl.solana.com/token)
