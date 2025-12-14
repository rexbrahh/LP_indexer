# Module Responsibilities

## Ingestor (`/ingestor`)
- **geyser/** – Yellowstone client & processor (replay window, Raydium swap decoding, JetStream publish path) plus failover orchestration.
- **helius/** – LaserStream/WS fallback (future wiring; adapter hooks ready).
- **common/** – Slot→timestamp cache, replay markers, shared telemetry helpers.
- **Outputs:** `BlockHead`, `TxMeta`, canonical `SwapEvent` protobufs via JetStream.

## Decoder (`/decoder`)
- **raydium/** – Parses Raydium CLMM instructions/logs into canonical `SwapEvent`.
- **orca_whirlpool/** – CLMM price math, sqrt price fields, tick/liquidity capture.
- **meteora/** – DLMM/CPMM adapter (future work).
- **Shared:** `decoder/common` for mint metadata, canonical pair resolver, fixed-point tests.

## State compute (`/state`)
- **candle_cpp/** – C++20 engine; sharded window store, timing wheel finalizer, provisional flag handling, pluggable publisher API (defaults to in-memory; JetStream wiring pending).
- **price_cpp/** – Fixed-point helpers, u128 math (future).
- Emits `dex.sol.candle.pool.*` and `dex.sol.candle.pair.*` subjects via publisher interface when hooked up to NATS.

## Sinks (`/sinks`)
- **nats/** – Publisher scaffolding (config/env parsing, API stubs, TODO for JetStream wiring).
- **clickhouse/** – Batch writer with retry/backoff; tests assert insert batching.
- **parquet/** – Cold storage writer scaffold (config/env parsing, TODO for parquet batching).

## Backfill (`/backfill`)
- **substreams/** – Manifest and notes for StreamingFast modules (scaffold).
- **orchestrator/** – Range scheduler + sink invoker scaffold (config env, stub runner).

## API (`/api`)
- **http/** – chi server, Redis cache wrapper, `/healthz`, `/v1/pool/:id`, `/v1/pool/:id/candles` endpoints.
- **grpc/** – Placeholder for internal low-latency API.

## Bridge (`/bridge`)
- Temporary service to mirror `dex.sol.*` subjects to legacy markets (scaffold: config/env parsing, stubbed runner).

## Ops (`/ops`)
- **jetstream/** – Stream & consumer JSON, CLI docs, verification script.
- **clickhouse/** – DDL scripts for trades, snapshots, OHLCV, wallet heuristics.
- **dashboards/** – Grafana definitions (TODO).

## Docs (`/docs`)
- Product spec, expectations, cutover runbook, observability guide, etc. (replenished).

## Legacy (`/legacy`)
- Holds existing Rust market-data binary while in parallel run; decommission after cutover.
