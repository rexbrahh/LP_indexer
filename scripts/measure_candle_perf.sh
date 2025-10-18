#!/usr/bin/env bash
set -euo pipefail

TRADES=${1:-50000}
CSV=${CSV_PATH:-$(mktemp -t swaps_perf.XXXXXX.csv)}
python3 - <<PY <<END "${TRADES}" "${CSV}"
import csv
import sys
trades = int(sys.argv[1])
path = sys.argv[2]
pair = "SOL_USDC"
base_ts = 1700000000
with open(path, 'w', newline='') as f:
    writer = csv.writer(f)
    writer.writerow(['pair_id','timestamp','price','base_amount','quote_amount'])
    for i in range(trades):
        ts = base_ts + (i // 10)
        price = 100.0 + (i % 100) * 0.01
        base = 1.0
        quote = base * price
        writer.writerow([pair, ts, price, base, quote])
print(path)
END
PY "$TRADES" "$CSV"
