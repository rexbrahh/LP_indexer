# Candle E2E CI Guide

This document describes how the nightly GitHub Actions workflow exercises the candle pipeline.

## Workflow Overview

Workflow file: `.github/workflows/candle-e2e.yml`

Steps executed:

1. **NATS JetStream** (`nats:2.10-alpine`) starts with JetStream enabled.
2. **ClickHouse server** (`clickhouse/clickhouse-server:24`) starts; we seed schema using `ops/clickhouse/all.sql`.
3. **MinIO** provides S3 API for the Parquet writer; a `dex-parquet` bucket is created via AWS CLI.
4. **Seed JetStream stream** `DEX` via `nats stream add`.
5. **Run harness**: `make candle-e2e INPUT=fixtures/swaps_sample.csv` which:
   - Builds `candle_replay`
   - Runs `cmd/candles` bridge with ClickHouse & Parquet target env vars
   - Replays swaps → candles into JetStream
   - Queries ClickHouse for row count
   - Lists S3 bucket to ensure parquet output exists
6. **Parity fixture**: `make candle-e2e INPUT=fixtures/swaps_parity.csv` then `scripts/check_candle_parity.sh`
6. Workflow prints ClickHouse count and S3 listing for auditing.

## Environment Variables

- `PARQUET_ENDPOINT`, `PARQUET_BUCKET`, `PARQUET_ACCESS_KEY`, `PARQUET_SECRET_KEY`
- `NATS_URL`, `NATS_STREAM`, `SUBJECT_ROOT`
- `CLICKHOUSE_DSN`, `CLICKHOUSE_DB`

## Nightly Schedule

- Cron: `0 6 * * *` (daily at 06:00 UTC)
- Manual trigger via `workflow_dispatch`

## Failure Recovery

- Harness outputs the failing command/logs.
- Use `scripts/run_candle_e2e.sh` locally to reproduce.

---

# Sink E2E Harness

- Workflow: `.github/workflows/sink-e2e.yml`
- Local run: `make sink-e2e INPUT=fixtures/sink_sample.json`

## What it does

1. Boots ClickHouse, MinIO, and NATS JetStream (CI services or local Docker).
2. Launches the ClickHouse and Parquet sink services (`cmd/sink/clickhouse`, `cmd/sink/parquet`).
3. Replays sample swap events via `cmd/tools/sinkreplay`, covering provisional, finalized, and undo flows.
4. Asserts:
   - ClickHouse `trades` table receives finalized/undo rows.
   - Parquet objects land under `s3://dex-parquet/timeframe=/scope=/date=`.

## Environment Variables

```
NATS_URL, NATS_STREAM, SUBJECT_ROOT
CLICKHOUSE_DSN, CLICKHOUSE_DB, CLICKHOUSE_TRADES_TABLE
PARQUET_ENDPOINT, PARQUET_BUCKET, PARQUET_ACCESS_KEY, PARQUET_SECRET_KEY
```

## Expected Output

- `clickhouse-client` query listing swaps with `provisional` and `is_undo` flags.
- `aws s3 ls` showing newly written Parquet files.

If the harness stalls, inspect `/tmp/clickhouse-sink.log` and `/tmp/parquet-sink.log` for sink errors.
