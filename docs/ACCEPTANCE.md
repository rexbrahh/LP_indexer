# Acceptance Criteria

These are the go/no-go gates captured in the locked spec.

## Ingestors
- Yellowstone client publishes provisional (confirmed) and finalized events, replaying the last 64 slots on reconnect.
- Helius fallback mirrors the message schema and can assume ownership automatically when the primary stream drops.

## Decoders
- Golden vectors cover tx→swap translation, sqrt→price conversion (CLMM), CPMM reserve mid-price, and decimal scaling.
- Canonical pair resolver enforces USDC → USDT → SOL quoting priority; fixtures for Raydium and Orca pass.

## Candle Engine
- Timing wheel finalizes 1s/1m/5m/1h/1d windows, flipping `provisional=false`, emitting corrections when late data arrives.
- Performance target: ≥100k swap events/s across 8 shards with P95 update latency < 800 ms.

## Sinks
- ClickHouse writer performs idempotent upserts (`(slot,sig,index)` PK) with retry/backoff and no duplicates.
- Parquet writer generates hourly/daily rolls to S3/MinIO for cold storage parity.

## Backfill
- Substreams playback for a known day matches the live pipeline: `|ΔOHLC| ≤ 0.1%`, `|Δvolume| ≤ 0.1%`, `|Δtrades| ≤ 0.1%`.
- Range checkpoints persist (Postgres/ClickHouse) for restart safety.

## APIs
- HTTP/GraphQL endpoints expose latest price, pool snapshot, candles, wallet heuristics; Redis cache optional but gracefully disabled.
- Legacy bridge mirrors new subjects until cutover completes.

## Observability & Cutover
- Metrics listed in `docs/OBSERVABILITY.md` exported; dashboards and alerts committed.
- Cutover runbook (`docs/CUTOVER.md`) executed: bootstrap → dark launch → shadow compare → flip → retire.
- Legacy market-data service decommissioned once parity holds for seven consecutive days.
