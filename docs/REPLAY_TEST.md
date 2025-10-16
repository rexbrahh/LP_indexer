# Replay Test Harness

## Purpose
Validate that ingest → decode → candle pipeline produces the same results as legacy feeds when replaying recorded Solana slots.

## Inputs
- Slot range JSON (start/stop)
- Recorded transactions + account state for Raydium/Orca/Meteora pools
- Legacy aggregates for comparison (volumes, OHLC)

## Steps
1. Feed recorded stream to ingestor/decoder (mock JetStream or in-memory channel).
2. Run candle engine on generated `SwapEvent`s.
3. Aggregate results and compare vs legacy metrics within tolerance.

## Acceptance Thresholds
- `|ΔOHLC| ≤ 0.1%` (or within one CLMM tick)
- `|Δvolume| ≤ 0.1%`
- `|Δtrades| ≤ 0.1%`

## Tooling (todo)
- CLI harness to load fixtures and emit diff report.
- Optionally write results to ClickHouse for analysis.
