#!/usr/bin/env bash
set -euo pipefail

INPUT=${1:-fixtures/sink_sample.json}

NATS_URL=${NATS_URL:-nats://127.0.0.1:4222}
NATS_STREAM=${NATS_STREAM:-DEX}
SUBJECT_ROOT=${SUBJECT_ROOT:-dex.sol}

CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-tcp://127.0.0.1:9000}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-default}
CLICKHOUSE_TRADES_TABLE=${CLICKHOUSE_TRADES_TABLE:-trades}

PARQUET_ENDPOINT=${PARQUET_ENDPOINT:-http://127.0.0.1:9000}
PARQUET_BUCKET=${PARQUET_BUCKET:-dex-parquet}
PARQUET_ACCESS_KEY=${PARQUET_ACCESS_KEY:-minio}
PARQUET_SECRET_KEY=${PARQUET_SECRET_KEY:-minio123}

export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-${PARQUET_ACCESS_KEY}}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-${PARQUET_SECRET_KEY}}

CH_CONSUMER=${CH_SINK_CONSUMER:-clickhouse-sink-e2e}
PQ_CONSUMER=${PARQUET_CONSUMER:-parquet-sink-e2e}

cleanup() {
  local code=$?
  if [[ -n "${CH_PID:-}" ]]; then
    kill "${CH_PID}" 2>/dev/null || true
    wait "${CH_PID}" 2>/dev/null || true
  fi
  if [[ -n "${PQ_PID:-}" ]]; then
    kill "${PQ_PID}" 2>/dev/null || true
    wait "${PQ_PID}" 2>/dev/null || true
  fi
  exit $code
}
trap cleanup EXIT

if command -v clickhouse-client >/dev/null 2>&1; then
  clickhouse-client --query "CREATE DATABASE IF NOT EXISTS ${CLICKHOUSE_DB}" >/dev/null
  clickhouse-client < ops/clickhouse/all.sql >/dev/null
  clickhouse-client --query "TRUNCATE TABLE ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE}" >/dev/null
else
  echo "clickhouse-client not found" >&2
  exit 1
fi

if command -v aws >/dev/null 2>&1; then
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 rb "s3://${PARQUET_BUCKET}" --force >/dev/null 2>&1 || true
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 mb "s3://${PARQUET_BUCKET}" >/dev/null
else
  echo "aws CLI not found" >&2
  exit 1
fi

export CH_SINK_NATS_URL="${NATS_URL}"
export CH_SINK_STREAM="${NATS_STREAM}"
export CH_SINK_SUBJECT_ROOT="${SUBJECT_ROOT}"
export CH_SINK_CONSUMER="${CH_CONSUMER}"
export CH_SINK_PULL_BATCH=${CH_SINK_PULL_BATCH:-64}
export CH_SINK_PULL_TIMEOUT_MS=${CH_SINK_PULL_TIMEOUT_MS:-500}
export CH_SINK_FLUSH_INTERVAL_MS=${CH_SINK_FLUSH_INTERVAL_MS:-500}
export CH_SINK_DSN="${CLICKHOUSE_DSN}"
export CH_SINK_DATABASE="${CLICKHOUSE_DB}"
export CH_SINK_TRADES_TABLE="${CLICKHOUSE_TRADES_TABLE}"

export PARQUET_NATS_URL="${NATS_URL}"
export PARQUET_NATS_STREAM="${NATS_STREAM}"
export PARQUET_SUBJECT_ROOT="${SUBJECT_ROOT}"
export PARQUET_CONSUMER="${PQ_CONSUMER}"
export PARQUET_PULL_BATCH=${PARQUET_PULL_BATCH:-64}
export PARQUET_PULL_TIMEOUT_MS=${PARQUET_PULL_TIMEOUT_MS:-500}
export PARQUET_FLUSH_INTERVAL_S=${PARQUET_FLUSH_INTERVAL_S:-2}
export PARQUET_ENDPOINT
export PARQUET_BUCKET
export PARQUET_ACCESS_KEY
export PARQUET_SECRET_KEY
export PARQUET_BATCH_ROWS=${PARQUET_BATCH_ROWS:-1000}
export PARQUET_REGION=${PARQUET_REGION:-us-east-1}
export S3_ENDPOINT="${PARQUET_ENDPOINT}"
export S3_BUCKET="${PARQUET_BUCKET}"
export S3_ACCESS_KEY="${PARQUET_ACCESS_KEY}"
export S3_SECRET_KEY="${PARQUET_SECRET_KEY}"

(go run ./cmd/sink/clickhouse >/tmp/clickhouse-sink.log 2>&1 &) && CH_PID=$!
(go run ./cmd/sink/parquet >/tmp/parquet-sink.log 2>&1 &) && PQ_PID=$!

sleep 2

go run ./cmd/tools/sinkreplay --input "${INPUT}" --nats-url "${NATS_URL}" --subject-root "${SUBJECT_ROOT}" --delay-ms 50

sleep 3

kill "${CH_PID}" 2>/dev/null || true
wait "${CH_PID}" 2>/dev/null || true
CH_PID=""

kill "${PQ_PID}" 2>/dev/null || true
wait "${PQ_PID}" 2>/dev/null || true
PQ_PID=""

EXPECTED=$(jq -r '[.[] | select(.type=="swap" and (.provisional==false or (.provisional==null and .is_undo==true))) ] | length' "${INPUT}")

actual_rows=$(clickhouse-client --query "SELECT count() FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} WHERE provisional = 0" | tr -d '\n')
undo_rows=$(clickhouse-client --query "SELECT count() FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} WHERE is_undo = 1" | tr -d '\n')

if [[ "${actual_rows}" != "${EXPECTED}" ]]; then
  echo "expected ${EXPECTED} finalized rows, got ${actual_rows}" >&2
  exit 1
fi

expected_undo=$(jq -r '[.[] | select(.type=="swap" and .is_undo==true)] | length' "${INPUT}")
if [[ "${undo_rows}" != "${expected_undo}" ]]; then
  echo "expected ${expected_undo} undo rows, got ${undo_rows}" >&2
  exit 1
fi

OBJECTS=$(aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive | wc -l | tr -d ' ')
if [[ "${OBJECTS}" -lt 1 ]]; then
  echo "no parquet objects written" >&2
  exit 1
fi

aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive
clickhouse-client --query "SELECT slot, sig, index, provisional, is_undo FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} ORDER BY slot, sig, index"
