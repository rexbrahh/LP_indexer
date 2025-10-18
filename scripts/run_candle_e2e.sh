#!/usr/bin/env bash
set -euo pipefail

INPUT=${1:-fixtures/swaps_sample.csv}
NATS_URL=${NATS_URL:-nats://127.0.0.1:4222}
NATS_STREAM=${NATS_STREAM:-DEX}
SUBJECT_ROOT=${SUBJECT_ROOT:-dex.sol}
CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-tcp://127.0.0.1:9000}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-default}
CLICKHOUSE_CANDLES_TABLE=${CLICKHOUSE_CANDLES_TABLE:-candles}
DURABLE=${DURABLE:-candle_e2e}
PULL_WAIT_MS=${PULL_WAIT_MS:-200}
BATCH=${BATCH:-64}
BRIDGE_TIMEOUT=${BRIDGE_TIMEOUT:-10}

if ! command -v timeout >/dev/null 2>&1; then
  echo "timeout command not found (coreutils)." >&2
  exit 1
fi

cmake -S state/candle_cpp -B build/candle_cpp >/dev/null
cmake --build build/candle_cpp >/dev/null

clickhouse-client --query "CREATE TABLE IF NOT EXISTS ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE} (timestamp DateTime64(9, 'UTC'), pool_id String, open Float64, high Float64, low Float64, close Float64, volume Float64) ENGINE = MergeTree ORDER BY (pool_id, timestamp)" >/dev/null
clickhouse-client --query "TRUNCATE TABLE ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}" >/dev/null

CANDLE_SUBJECT="${SUBJECT_ROOT}.candle.>"

go run ./cmd/candles \
  -nats "${NATS_URL}" \
  -stream "${NATS_STREAM}" \
  -subject "${CANDLE_SUBJECT}" \
  -durable "${DURABLE}" \
  -batch "${BATCH}" \
  -pull-wait "${PULL_WAIT_MS}" \
  -clickhouse-dsn "${CLICKHOUSE_DSN}" \
  -clickhouse-db "${CLICKHOUSE_DB}" \
  -clickhouse-candles "${CLICKHOUSE_CANDLES_TABLE}" \
  > /dev/null 2>&1 &
BRIDGE_PID=$!

timeout "${BRIDGE_TIMEOUT}" ./build/candle_cpp/candle_replay \
  --input "${INPUT}" \
  --nats-url "${NATS_URL}" \
  --stream "${NATS_STREAM}" \
  --subject-root "${SUBJECT_ROOT}"

sleep 2
kill $BRIDGE_PID >/dev/null 2>&1 || true
wait $BRIDGE_PID 2>/dev/null || true

clickhouse-client --query "SELECT count() AS candles_inserted FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}" || true

if command -v aws >/dev/null 2>&1 && [ -n "${PARQUET_BUCKET:-}" ] && [ -n "${PARQUET_ENDPOINT:-}" ]; then
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive || true
fi
