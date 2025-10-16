# Acceptance Criteria

## Ingestors
- Yellowstone client reconnects automatically, replays last 64 slots without duplicate downstream emissions.
- Fallback (Helius) can be toggled on failure and publishes identical schemas.

## Decoders
- Raydium & Orca normalization tests pass with recorded fixtures.
- Canonical pair resolver obeys USD/SOL priority; price/volume math validated.

## Candle Engine
- Finalization timing wheel emits candles with `provisional=false` and clears state.
- Benchmarks demonstrate ≥100k events/s across 8 shards with p95 update latency < 800 ms.

## Sinks
- ClickHouse writer batches inserts, respects MaxRetries/backoff, and leaves deterministic state.
- Parquet writer produces hourly/daily rolls to S3/MinIO (stub to be implemented).

## Backfill
- Substreams replay matches live pipeline within tolerance; checkpoints persisted.

## APIs
- HTTP service exposes `/healthz`, `/v1/pool/:id`, `/v1/pool/:id/candles?tf=<tf>` with Redis cache fallback.
- gRPC service stub compiles; low-latency responses planned for later milestone.

## Observability
- `make ops.jetstream.verify` passes before enabling consumers.
- Prometheus metrics instrumented across ingestors, candle engine, sinks before GA.

Meeting these criteria signals readiness to retire the legacy market-data service.
