#!/usr/bin/env bash
set -euo pipefail

INPUT=${1:-fixtures/sink_sample.json}

NATS_URL=${NATS_URL:-nats://127.0.0.1:4222}
NATS_STREAM=${NATS_STREAM:-DEX}
SUBJECT_ROOT=${SUBJECT_ROOT:-dex.sol}

CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-tcp://127.0.0.1:9000}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-default}
CLICKHOUSE_TRADES_TABLE=${CLICKHOUSE_TRADES_TABLE:-trades}

PARQUET_ENDPOINT=${PARQUET_ENDPOINT:-http://127.0.0.1:9005}
PARQUET_BUCKET=${PARQUET_BUCKET:-dex-parquet}
PARQUET_ACCESS_KEY=${PARQUET_ACCESS_KEY:-minioadmin}
PARQUET_SECRET_KEY=${PARQUET_SECRET_KEY:-minioadmin}
PARQUET_REGION=${PARQUET_REGION:-us-east-1}

export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-${PARQUET_ACCESS_KEY}}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-${PARQUET_SECRET_KEY}}
export AWS_REGION=${AWS_REGION:-${PARQUET_REGION}}

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

CLICKHOUSE_CMD=()
if command -v clickhouse-client >/dev/null 2>&1; then
  CLICKHOUSE_CMD=(clickhouse-client)
elif command -v clickhouse >/dev/null 2>&1; then
  CLICKHOUSE_CMD=(clickhouse client)
elif [[ -d /opt/homebrew/Caskroom/clickhouse ]]; then
  for candidate in /opt/homebrew/Caskroom/clickhouse/*/clickhouse-macos-aarch64; do
    if [[ -x "${candidate}" ]]; then
      CLICKHOUSE_CMD=("${candidate}" client)
      break
    fi
  done
fi

if [[ ${#CLICKHOUSE_CMD[@]} -eq 0 ]]; then
  echo "clickhouse client binary not found" >&2
  exit 1
fi

"${CLICKHOUSE_CMD[@]}" --query "CREATE DATABASE IF NOT EXISTS ${CLICKHOUSE_DB}" >/dev/null
"${CLICKHOUSE_CMD[@]}" --query "DROP TABLE IF EXISTS ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE}" >/dev/null
"${CLICKHOUSE_CMD[@]}" < ops/clickhouse/all.sql >/dev/null
"${CLICKHOUSE_CMD[@]}" --query "TRUNCATE TABLE ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE}" >/dev/null

if command -v aws >/dev/null 2>&1; then
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 rb "s3://${PARQUET_BUCKET}" --force >/dev/null 2>&1 || true
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 mb "s3://${PARQUET_BUCKET}" --region "${PARQUET_REGION}" >/dev/null
else
  echo "aws CLI not found" >&2
  exit 1
fi

if ! command -v duckdb >/dev/null 2>&1; then
  echo "duckdb CLI not found" >&2
  exit 1
fi

export CH_SINK_NATS_URL="${NATS_URL}"
export CH_SINK_NATS_STREAM="${NATS_STREAM}"
export CH_SINK_SUBJECT_ROOT="${SUBJECT_ROOT}"
export CH_SINK_CONSUMER="${CH_CONSUMER}"
export CH_SINK_PULL_BATCH=${CH_SINK_PULL_BATCH:-64}
export CH_SINK_PULL_TIMEOUT_MS=${CH_SINK_PULL_TIMEOUT_MS:-500}
export CH_SINK_FLUSH_INTERVAL_MS=${CH_SINK_FLUSH_INTERVAL_MS:-500}
export CH_SINK_DSN="${CLICKHOUSE_DSN}"
export CH_SINK_DATABASE="${CLICKHOUSE_DB}"
export CH_SINK_TRADES_TABLE="${CLICKHOUSE_TRADES_TABLE}"
export CH_SINK_CANDLES_TABLE=${CH_SINK_CANDLES_TABLE:-ohlcv_1s}

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

go run ./cmd/sink/clickhouse >/tmp/clickhouse-sink.log 2>&1 &
CH_PID=$!
go run ./cmd/sink/parquet >/tmp/parquet-sink.log 2>&1 &
PQ_PID=$!

sleep 2

go run ./cmd/tools/sinkreplay --input "${INPUT}" --nats-url "${NATS_URL}" --subject-root "${SUBJECT_ROOT}" --delay-ms 50

sleep 3

kill -s INT "${CH_PID}" 2>/dev/null || kill "${CH_PID}" 2>/dev/null || true
wait "${CH_PID}" 2>/dev/null || true
CH_PID=""

kill -s INT "${PQ_PID}" 2>/dev/null || kill "${PQ_PID}" 2>/dev/null || true
wait "${PQ_PID}" 2>/dev/null || true
PQ_PID=""

EXPECTED=$(jq -r '[.[] | select(.type=="swap" and (.provisional==false or (.provisional==null and .is_undo==true))) ] | length' "${INPUT}")

actual_rows=$("${CLICKHOUSE_CMD[@]}" --query "SELECT count() FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} WHERE provisional = 0" | tr -d '\n')
undo_rows=$("${CLICKHOUSE_CMD[@]}" --query "SELECT count() FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} WHERE is_undo = 1" | tr -d '\n')

if [[ "${actual_rows}" != "${EXPECTED}" ]]; then
  echo "expected ${EXPECTED} finalized rows, got ${actual_rows}" >&2
  exit 1
fi

expected_undo=$(jq -r '[.[] | select(.type=="swap" and .is_undo==true)] | length' "${INPUT}")
if [[ "${undo_rows}" != "${expected_undo}" ]]; then
  echo "expected ${expected_undo} undo rows, got ${undo_rows}" >&2
  exit 1
fi

expected_swaps=$(mktemp)
jq '[.[] | select(.type=="swap") | {
  slot,
  signature,
  idx: (.index // 0),
  program_id,
  pool_id,
  base_in: (.base_in // 0),
  base_out: (.base_out // 0),
  quote_in: (.quote_in // 0),
  quote_out: (.quote_out // 0),
  reserves_base: (.reserves_base // 0),
  reserves_quote: (.reserves_quote // 0),
  fee_bps: (.fee_bps // 0),
  provisional: (if (.provisional // false) then 1 else 0 end),
  is_undo: (if (.is_undo // false) then 1 else 0 end)
}] | sort_by([.slot, .signature, .idx, .provisional, .is_undo])' "${INPUT}" > "${expected_swaps}"

expected_candle_count=$(jq '[.[] | select(.type=="candle")] | length' "${INPUT}")
expected_timeframes=$(jq -c '[.[] | select(.type=="candle") | (if ((.timeframe // "") == "" then "unknown" else (.timeframe | ascii_downcase)))] | unique | sort' "${INPUT}")
expected_scopes=$(jq -c '[.[] | select(.type=="candle") | (if ((.pool_id // "") != "") then "pool" else "pair")] | unique | sort' "${INPUT}")

actual_swaps=$(mktemp)
"${CLICKHOUSE_CMD[@]}" --query "SELECT slot, sig AS signature, idx, program_id, pool_id, toUInt64(base_in) AS base_in, toUInt64(base_out) AS base_out, toUInt64(quote_in) AS quote_in, toUInt64(quote_out) AS quote_out, toUInt64(reserves_base) AS reserves_base, toUInt64(reserves_quote) AS reserves_quote, toUInt16(fee_bps) AS fee_bps, toUInt8(provisional) AS provisional, toUInt8(is_undo) AS is_undo FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} ORDER BY slot, sig, idx, provisional, is_undo FORMAT JSONEachRow" | \
  jq -s 'def toflag($v):
    (if ($v|type) == "number" then ($v|tonumber)
     elif ($v|type) == "string" then ($v|tonumber)
     elif ($v|type) == "boolean" then (if $v then 1 else 0 end)
     else 0 end);
  map(.provisional = toflag(.provisional) | .is_undo = toflag(.is_undo))
  | sort_by([.slot, .signature, .idx, .provisional, .is_undo])' > "${actual_swaps}"

if ! diff -u "${expected_swaps}" "${actual_swaps}" >/dev/null; then
  echo "ClickHouse swap rows do not match fixture" >&2
  diff -u "${expected_swaps}" "${actual_swaps}" || true
  exit 1
fi

object_listing=$(mktemp)
aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive > "${object_listing}"
OBJECTS=$(wc -l < "${object_listing}" | tr -d ' ')
if [[ "${OBJECTS}" -lt 1 ]]; then
  echo "no parquet objects written" >&2
  exit 1
fi

tmp_parquet_dir=$(mktemp -d)
awk '{print $4}' "${object_listing}" | while read -r key; do
  if [[ -z "${key}" ]]; then
    continue
  fi
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 cp "s3://${PARQUET_BUCKET}/${key}" "${tmp_parquet_dir}/$(basename "${key}")" >/dev/null
done

parquet_summary=$(mktemp)
go run ./cmd/tools/parquetinspect --pattern "${tmp_parquet_dir}/*.parquet" > "${parquet_summary}"

total_rows=$(jq -r '.total_rows' "${parquet_summary}")
empty_timeframe=$(jq -r '.empty_timeframe' "${parquet_summary}")
empty_scope=$(jq -r '.empty_scope' "${parquet_summary}")
invalid_scope=$(jq -r '.invalid_scope' "${parquet_summary}")
missing_window_start=$(jq -r '.missing_window_start' "${parquet_summary}")
neg_trades=$(jq -r '.negative_trades' "${parquet_summary}")
actual_timeframes=$(jq -c '((.unique_timeframes // []) | map(ascii_downcase) | sort)' "${parquet_summary}")
actual_scopes=$(jq -c '((.unique_scopes // []) | map(ascii_downcase) | sort)' "${parquet_summary}")

if [[ "${total_rows}" -ne "${expected_candle_count}" ]]; then
  echo "parquet row count mismatch (expected ${expected_candle_count}, got ${total_rows})" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${empty_timeframe}" -ne 0 ]]; then
  echo "parquet rows missing timeframe" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${empty_scope}" -ne 0 ]]; then
  echo "parquet rows missing scope" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${invalid_scope}" -ne 0 ]]; then
  echo "parquet rows have invalid scope" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${missing_window_start}" -ne 0 ]]; then
  echo "parquet rows missing window_start" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${neg_trades}" -ne 0 ]]; then
  echo "parquet rows have negative trade counts" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${actual_timeframes}" != "${expected_timeframes}" ]]; then
  echo "parquet timeframes ${actual_timeframes} do not match expected ${expected_timeframes}" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi
if [[ "${actual_scopes}" != "${expected_scopes}" ]]; then
  echo "parquet scopes ${actual_scopes} do not match expected ${expected_scopes}" >&2
  cat "${parquet_summary}" >&2
  exit 1
fi

aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive
"${CLICKHOUSE_CMD[@]}" --query "SELECT slot, sig, idx, provisional, is_undo FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_TRADES_TABLE} ORDER BY slot, sig, idx"
