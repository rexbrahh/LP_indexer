#!/usr/bin/env bash
set -euo pipefail

INPUT=${1:-fixtures/swaps_sample.csv}
NATS_URL=${NATS_URL:-nats://127.0.0.1:4222}
NATS_STREAM=${NATS_STREAM:-DEX}
SUBJECT_ROOT=${SUBJECT_ROOT:-dex.sol}
CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-tcp://127.0.0.1:9000}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-default}
CLICKHOUSE_CANDLES_TABLE=${CLICKHOUSE_CANDLES_TABLE:-candles}
CLICKHOUSE_CLIENT=${CLICKHOUSE_CLIENT:-clickhouse-client}
DURABLE=${DURABLE:-candle_e2e}
PULL_WAIT_MS=${PULL_WAIT_MS:-200}
BATCH=${BATCH:-64}
BRIDGE_TIMEOUT=${BRIDGE_TIMEOUT:-10}

if ! command -v timeout >/dev/null 2>&1; then
  COREUTILS_GNUBIN=/opt/homebrew/opt/coreutils/libexec/gnubin
  if [ -d "$COREUTILS_GNUBIN" ]; then
    PATH="$COREUTILS_GNUBIN:$PATH"
    export PATH
  fi
fi

if command -v timeout >/dev/null 2>&1; then
  TIMEOUT_BIN=timeout
elif command -v gtimeout >/dev/null 2>&1; then
  TIMEOUT_BIN=gtimeout
else
  echo "timeout command not found. Install coreutils to provide timeout or gtimeout." >&2
  exit 1
fi

if ! command -v "${CLICKHOUSE_CLIENT%% *}" >/dev/null 2>&1; then
  if command -v docker >/dev/null 2>&1; then
    CLICKHOUSE_CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'clickhouse' | head -n1 || true)
    if [ -n "$CLICKHOUSE_CONTAINER" ]; then
      CLICKHOUSE_CLIENT="docker exec -i $CLICKHOUSE_CONTAINER clickhouse-client"
    else
      echo "clickhouse-client not found and no running clickhouse container detected." >&2
      exit 1
    fi
  else
    echo "clickhouse-client not found and Docker unavailable to exec into a container." >&2
    exit 1
  fi
fi

PROTOBUF_PREFIX=${PROTOBUF_PREFIX:-}
if [ -z "$PROTOBUF_PREFIX" ]; then
  if [ -d /opt/homebrew/opt/protobuf ]; then
    PROTOBUF_PREFIX=/opt/homebrew/opt/protobuf
  elif [ -d /usr/local/opt/protobuf ]; then
    PROTOBUF_PREFIX=/usr/local/opt/protobuf
  fi
fi

CMAKE_ARGS=()
if [ -n "${PROTOBUF_PREFIX}" ]; then
  if [ -d "${PROTOBUF_PREFIX}/bin" ]; then
    PATH="${PROTOBUF_PREFIX}/bin:$PATH"
    export PATH
  fi
  if [ -d "${PROTOBUF_PREFIX}/lib/cmake/protobuf" ]; then
    CMAKE_ARGS+=("-DProtobuf_DIR=${PROTOBUF_PREFIX}/lib/cmake/protobuf")
  fi
fi

ABSL_PREFIX=${ABSL_PREFIX:-}
if [ -z "$ABSL_PREFIX" ]; then
  if [ -d /opt/homebrew/opt/abseil ]; then
    ABSL_PREFIX=/opt/homebrew/opt/abseil
  elif [ -d /usr/local/opt/abseil ]; then
    ABSL_PREFIX=/usr/local/opt/abseil
  fi
fi

if [ -n "$ABSL_PREFIX" ] && [ -d "${ABSL_PREFIX}/lib/cmake/absl" ]; then
  CMAKE_ARGS+=("-Dabsl_DIR=${ABSL_PREFIX}/lib/cmake/absl")
fi

cmake -S state/candle_cpp -B build/candle_cpp "${CMAKE_ARGS[@]}" >/dev/null
cmake --build build/candle_cpp >/dev/null

eval "$CLICKHOUSE_CLIENT --query \"CREATE TABLE IF NOT EXISTS ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE} (timestamp DateTime64(9, 'UTC'), pool_id String, open Float64, high Float64, low Float64, close Float64, volume Float64) ENGINE = MergeTree ORDER BY (pool_id, timestamp)\"" >/dev/null
eval "$CLICKHOUSE_CLIENT --query \"TRUNCATE TABLE ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}\"" >/dev/null

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

"${TIMEOUT_BIN}" "${BRIDGE_TIMEOUT}" ./build/candle_cpp/candle_replay \
  --input "${INPUT}" \
  --nats-url "${NATS_URL}" \
  --stream "${NATS_STREAM}" \
  --subject-root "${SUBJECT_ROOT}"

sleep 2
kill $BRIDGE_PID >/dev/null 2>&1 || true
wait $BRIDGE_PID 2>/dev/null || true

eval "$CLICKHOUSE_CLIENT --query \"SELECT count() AS candles_inserted FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}\"" || true

if command -v aws >/dev/null 2>&1 && [ -n "${PARQUET_BUCKET:-}" ] && [ -n "${PARQUET_ENDPOINT:-}" ]; then
  aws --endpoint-url "${PARQUET_ENDPOINT}" s3 ls "s3://${PARQUET_BUCKET}" --recursive || true
fi
