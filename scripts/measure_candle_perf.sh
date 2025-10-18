#!/usr/bin/env bash
set -euo pipefail

TRADES=${1:-50000}
CSV=${CSV_PATH:-$(mktemp -t swaps_perf.XXXXXX.csv)}
python3 - "$TRADES" "$CSV" <<'PY'
import csv
import sys
trades = int(sys.argv[1])
path = sys.argv[2]
pair = "SOL_USDC"
base_ts = 1700000000
with open(path, 'w', newline='') as f:
    writer = csv.writer(f)
    writer.writerow(['pair_id', 'timestamp', 'price', 'base_amount', 'quote_amount'])
    for i in range(trades):
        ts = base_ts + (i // 10)
        price = 100.0 + (i % 100) * 0.01
        base = 1.0
        quote = base * price
        writer.writerow([pair, ts, price, base, quote])
print(path)
PY

CLICKHOUSE_CLIENT=${CLICKHOUSE_CLIENT:-clickhouse-client}
CLICKHOUSE_DB=${CLICKHOUSE_DB:-default}
CLICKHOUSE_CANDLES_TABLE=${CLICKHOUSE_CANDLES_TABLE:-candles_perf}
CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-tcp://127.0.0.1:9000}

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

eval "$CLICKHOUSE_CLIENT --query \"CREATE TABLE IF NOT EXISTS ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE} (timestamp DateTime64(9, 'UTC'), pool_id String, open Float64, high Float64, low Float64, close Float64, volume Float64) ENGINE = MergeTree ORDER BY (pool_id, timestamp)\"" >/dev/null
eval "$CLICKHOUSE_CLIENT --query \"TRUNCATE TABLE ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}\"" >/dev/null

PROTOBUF_PREFIX=${PROTOBUF_PREFIX:-}
if [ -z "$PROTOBUF_PREFIX" ]; then
  if [ -d /opt/homebrew/opt/protobuf ]; then
    PROTOBUF_PREFIX=/opt/homebrew/opt/protobuf
  elif [ -d /usr/local/opt/protobuf ]; then
    PROTOBUF_PREFIX=/usr/local/opt/protobuf
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

CMAKE_ARGS=(-DOPENSSL_ROOT_DIR=/opt/homebrew/opt/openssl -DNATS_BUILD_STREAMING=OFF)
if [ -n "$PROTOBUF_PREFIX" ]; then
  if [ -d "${PROTOBUF_PREFIX}/bin" ]; then
    PATH="${PROTOBUF_PREFIX}/bin:$PATH"
    export PATH
  fi
  if [ -d "${PROTOBUF_PREFIX}/lib/cmake/protobuf" ]; then
    CMAKE_ARGS+=("-DProtobuf_DIR=${PROTOBUF_PREFIX}/lib/cmake/protobuf")
  fi
fi
if [ -n "$ABSL_PREFIX" ] && [ -d "${ABSL_PREFIX}/lib/cmake/absl" ]; then
  CMAKE_ARGS+=("-Dabsl_DIR=${ABSL_PREFIX}/lib/cmake/absl")
fi

cmake -S state/candle_cpp -B build/candle_cpp "${CMAKE_ARGS[@]}" >/dev/null
cmake --build build/candle_cpp >/dev/null
go build -o bridge-perf ./cmd/candles

NATS_URL=${NATS_URL:-nats://127.0.0.1:4222}
NATS_STREAM=${NATS_STREAM:-DEX}
SUBJECT_ROOT=${SUBJECT_ROOT:-dex.sol}
PARQUET_ENDPOINT=${PARQUET_ENDPOINT:-}
PARQUET_BUCKET=${PARQUET_BUCKET:-}
PARQUET_ACCESS_KEY=${PARQUET_ACCESS_KEY:-}
PARQUET_SECRET_KEY=${PARQUET_SECRET_KEY:-}

BRIDGE_LOG=$(mktemp -t bridge_log.XXXXXX)
trap 'kill $BRIDGE_PID 2>/dev/null || true; rm -f "$CSV" "$BRIDGE_LOG"' EXIT

START_TOTAL=$(date +%s%N)
CLICKHOUSE_CANDLES_TABLE=${CLICKHOUSE_CANDLES_TABLE} \
PARQUET_ENDPOINT=${PARQUET_ENDPOINT} \
PARQUET_BUCKET=${PARQUET_BUCKET} \
PARQUET_ACCESS_KEY=${PARQUET_ACCESS_KEY} \
PARQUET_SECRET_KEY=${PARQUET_SECRET_KEY} \
./bridge-perf -nats "$NATS_URL" -stream "$NATS_STREAM" -subject "${SUBJECT_ROOT}.candle.>" -durable candle_perf -batch 512 -pull-wait 200 -clickhouse-dsn "$CLICKHOUSE_DSN" -clickhouse-db "$CLICKHOUSE_DB" -clickhouse-candles "$CLICKHOUSE_CANDLES_TABLE" >"$BRIDGE_LOG" 2>&1 &
BRIDGE_PID=$!

sleep 1

START_REPLAY=$(date +%s%N)
./build/candle_cpp/candle_replay --input "$CSV" --nats-url "$NATS_URL" --stream "$NATS_STREAM" --subject-root "$SUBJECT_ROOT"
END_REPLAY=$(date +%s%N)

sleep 2
kill $BRIDGE_PID >/dev/null 2>&1 || true
wait $BRIDGE_PID 2>/dev/null || true
END_TOTAL=$(date +%s%N)

REPLAY_MS=$(((END_REPLAY - START_REPLAY)/1000000))
TOTAL_MS=$(((END_TOTAL - START_TOTAL)/1000000))
if [ "$REPLAY_MS" -eq 0 ]; then
  REPLAY_MS=1
fi
THROUGHPUT=$(python3 - <<'PY' $TRADES $REPLAY_MS
import sys
trades = float(sys.argv[1])
ms = float(sys.argv[2])
print(f"{(trades / (ms / 1000.0)):.2f}")
PY)

CLICKHOUSE_ROWS=$(eval "$CLICKHOUSE_CLIENT --query \"SELECT count() FROM ${CLICKHOUSE_DB}.${CLICKHOUSE_CANDLES_TABLE}\"")

printf 'Trades: %s\nReplay duration: %d ms\nTotal duration: %d ms\nThroughput: %s trades/sec\nClickHouse rows: %s\n' "$TRADES" "$REPLAY_MS" "$TOTAL_MS" "$THROUGHPUT" "$CLICKHOUSE_ROWS"
