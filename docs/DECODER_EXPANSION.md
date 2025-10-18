# Decoder Expansion Plan

## Orca Whirlpools (Integrated)
- Ingest processor now recognises Orca whirlpool program IDs, caches pool metadata,
  and publishes canonical `SwapEvent`s. Additional fixtures for regression tests
  remain on the TODO list.

## Meteora Pools
- **Inputs**: `decoder/meteora` scaffolding present but needs real account layout.
- **Work**:
  1. Collect pool account dumps and reference decoder implementation.
  2. Implement Meteora-specific decode path in processor (stub present).
  3. Add regression fixtures and metrics similar to Raydium.

## Pump.fun Native LP
- **Research Needed**:
  - Identify pool program/accounts and swap instruction layout.
  - Determine fee structure and vault association (likely dynamic per token).
- **Implementation Steps**:
  1. Document account schema from on-chain program or official docs.
  2. Build dedicated decoder (similar to Raydium) emitting canonical `SwapEvent`s.
  3. Integrate into ingest processor with program ID routing and metrics.

## Bonk Launchpad Liquidity
- **Research Needed**:
  - Confirm whether launchpad LP uses bespoke program or reuses existing AMM.
  - Gather sample transactions/pool accounts for decoding.
- **Implementation Steps**:
  1. Reverse-engineer swap instruction and pool state.
  2. Create decoder + fixtures; plug into ingest pipeline.
  3. Validate outputs against legacy data once available.

## Shared Tasks
- Expand processor to support multi-program routing with modular decoders.
- Maintain per-program metrics (`swaps_total`, `decode_errors_total`).
- Update failover handling to ensure both Geyser and Helius feeds use the same decoder set.
