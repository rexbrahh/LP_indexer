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

