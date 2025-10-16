# Day 0 Parallel Work Plan (6-hour Sprint)

Extracted directly from the locked spec. Each track should deliver a minimal, reviewable PR by end of day.

## Contracts & Infra
- [ ] Author `.proto` files (`dex.sol.v1` messages above) and wire `make proto-gen` (Go + C++ outputs).
- [ ] Commit JetStream stream/consumer JSON and `nats` CLI scripts.
- [ ] Apply ClickHouse DDL (`ops/clickhouse/*.sql`) and seed program registry.

## Ingestors (Go)
- [ ] `ingestor/geyser`: subscribe to Yellowstone gRPC, maintain slot→timestamp cache, emit provisional/final events, handle reorg undo.
- [ ] `ingestor/helius`: WS/gRPC fallback with identical message shapes; automatic takeover when Geyser drops.
- [ ] `decoder/*`: implement Raydium / Orca / Meteora adapters producing canonical `SwapEvent`s.

## Candle & State (C++20)
- [ ] Fixed-point utilities (`q32.32`, u128 helpers).
- [ ] Packed store + index + timing wheel scaffolding.
- [ ] CPMM/CLMM/DLMM price adapters.
- [ ] Window update, finalize, correction logic.
- [ ] NATS publisher + hooks for ClickHouse/Parquet sinks.

## Sinks (Go)
- [ ] ClickHouse batch writer (ReplacingMergeTree upserts, retry/backoff).
- [ ] Parquet roll writer (hourly/daily) to S3/MinIO.

## Backfill (Substreams + Orchestrator)
- [ ] Implement `map_swaps` / `map_pool_snapshots` modules covering Raydium, Orca, Meteora.
- [ ] Wire Substreams sink to ClickHouse; build Go orchestrator for range scheduling/checkpoints.

## API (Go)
- [ ] REST/GraphQL skeleton: latest price, pool snapshot, candles, wallet stats.
- [ ] Pagination + rate limit stubs.

## Bridge (TEMP)
- [ ] Mirror `dex.sol.*` subjects to legacy market-data topics; reuse idempotency key for dedupe.

## Observability
- [ ] Add Prometheus metrics per service; commit Grafana dashboards + alerts for slot lag, consumer lag, ack pending.

### Cadence
- Kickoff (30 min) to align on protobuf ownership and shared constants.
- Midpoint review (15 min) to surface blockers.
- End-of-day recap: demo progress, document follow-ups.

### Expectations
- Branch naming: `feat/<area>-<slug>` or `chore/<slug>`.
- Run `make fmt lint build` (and `scripts/build_candles.sh`) before submitting PRs.
- Update docs within same PR when behaviour or contracts change.
