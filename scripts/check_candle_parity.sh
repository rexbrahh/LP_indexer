#!/usr/bin/env bash
set -euo pipefail

EXPECTED="1700000000,SOL_USDC,100,105,98,98,2309
1700000060,SOL_USDC,110,110,110,110,220"

ACTUAL=$(clickhouse-client --query "SELECT toUnixTimestamp(timestamp) AS ts, pool_id, round(open,3), round(high,3), round(low,3), round(close,3), round(volume,3) FROM default.candles ORDER BY ts FORMAT CSV")

if [ "${ACTUAL}" != "${EXPECTED}" ]; then
  echo "candle parity check failed"
  echo "expected:\n${EXPECTED}"
  echo "actual:\n${ACTUAL}"
  exit 1
fi

echo "candle parity check passed"
