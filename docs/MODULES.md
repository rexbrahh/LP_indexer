# Module Responsibilities

## Ingestor (`/ingestor`)
- **geyser/** – Yellowstone client, handles replay window, token auth, reconnect loop.
- **helius/** – LaserStream/WS fallback (future stub).
- **common/** – Slot→timestamp cache, replay markers, shared telemetry helpers.
- **Outputs:** `BlockHead`, `TxMeta`, program-specific raw data forwarded to decoders.

## Decoder (`/decoder`)
- **raydium/** – Parses Raydium CLMM instructions/logs into canonical `SwapEvent`.
- **orca_whirlpool/** – CLMM price math, sqrt price fields, tick/liquidity capture.
- **meteora/** – DLMM/CPMM adapter (future work).
- **Shared:** `decoder/common` for mint metadata, canonical pair resolver, fixed-point tests.

## State compute (`/state`)
- **candle_cpp/** – C++20 engine; sharded window store, timing wheel finalizer, provisional flag handling.
- **price_cpp/** – Fixed-point helpers, u128 math (future). 
- Emits `dex.sol.candle.pool.*` and `dex.sol.candle.pair.*` subjects via NATS (stubbed now).

## Sinks (`/sinks`)
- **nats/** – Publisher utilities (TODO).
- **clickhouse/** – Batch writer with retry/backoff; tests assert insert batching.
- **parquet/** – Hourly roll writer to S3/MinIO (stub).

## Backfill (`/backfill`)
- **substreams/** – YAML for StreamingFast modules mapping swap/pool snapshots.
- **orchestrator/** – Range scheduler, sink invoker (stub).

## API (`/api`)
- **http/** – chi server, Redis cache wrapper, `/healthz`, `/v1/pool/:id`, `/v1/pool/:id/candles` endpoints.
- **grpc/** – Placeholder for internal low-latency API.

## Bridge (`/bridge`)
- Temporary service to mirror `dex.sol.*` subjects to legacy markets (skeleton).

## Ops (`/ops`)
- **jetstream/** – Stream & consumer JSON, CLI docs, verification script.
- **clickhouse/** – DDL scripts (future commit).
- **dashboards/** – Grafana definitions (TODO).

## Docs (`/docs`)
- Product spec, expectations, cutover runbook, observability guide, etc. (replenished).

## Legacy (`/legacy`)
- Holds existing Rust market-data binary while in parallel run; decommission after cutover.
