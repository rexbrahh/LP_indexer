# Cutover Plan – Legacy Market-Data ➜ Solana Liquidity Indexer

## Phase A – Dark Launch
1. Deploy JetStream stream/consumer via `make ops.jetstream.init`.
2. Apply ClickHouse schema changes (`make ops.clickhouse.apply`) so new reserve/fee columns exist.
3. Start Yellowstone + Helius ingestors with bridge forwarding to legacy subjects.
4. Launch sink services:
   - `go run ./cmd/sink/clickhouse` (set `CH_SINK_*` env vars).
   - `go run ./cmd/sink/parquet` (set `PARQUET_*` and S3 credentials).
5. Keep legacy Rust market-data service running for dashboards.

## Phase B – Shadow Compare (≥7 days)
- Compare per-pool swaps and OHLCV versus legacy aggregates (alert on |Δ| > 1% volume or >0.1% trades).
- Track JetStream consumer lag, redelivery, ack pending; ensure dedup window stays quiet.
- Validate sinks:
  - ClickHouse: check `SELECT count() FROM trades WHERE provisional=0` grows monotically; spot check undo rows (`is_undo=1`).
  - Parquet: confirm daily objects land in S3 with both `scope=pair` and `scope=pool` partitions.

## Phase C – Flip Read Paths
- Point internal services to `dex.sol.*` or the new HTTP/GraphQL endpoints.
- Maintain bridge for one additional safety week.

## Phase D – Retire Legacy
- Shut down Rust market-data binary and bridge.
- Remove legacy NATS subjects after downstream sign-off.

## Success Criteria
- Provisional publish p95 < 800 ms, finalized p95 < 20 s.
- Backfill throughput ≥ 1e6 swaps/hour/node.
- Decoder error rate < 1e-6/log, dedup drops < 1e-4.

## Rollback
- If variance thresholds exceed limits for >2 hours, shift consumers back to legacy, pause new ingestors, inspect decoder math & timestamp cache.
