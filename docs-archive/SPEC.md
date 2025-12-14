# Solana Liquidity Indexer – Locked Specification (v1)

This document mirrors the reference spec that kicked off the project. Treat it as the canonical contract for v1 scope.

---

## 1. Scope
- **Chain:** Solana mainnet-beta (`chain_id = 501`)
- **Realtime Sources:** Primary Yellowstone Geyser gRPC (Chainstack). Fallback Helius LaserStream/WS. Webhooks only for side effects.
- **Backfill:** StreamingFast Substreams → ClickHouse + daily Parquet.
- **DEX Coverage:** Raydium AMM, Orca Whirlpools (CLMM), Meteora (DLMM/CPMM).
- **Timeframes:** 1s, 1m, 5m, 1h, 1d.
- **Finality Policy:** Publish provisional at `confirmed`, finalize when slot reaches `finalized`.
- **Bus & Encoding:** NATS JetStream with protobuf payloads (`dex.sol.v1.*`), u128 modeled as `{hi, lo}`.
- **Language Split:** Go for ingest, APIs, sinks, orchestration. C++20 for hot compute (candles, pricing/state).
- **Pricing Anchors:** USDC pegged to $1 with depeg guard; SOL priced via VWAP vs USDC.

---

## 2. Repository Layout
```
solana-liquidity-indexer/
  /proto/                  # .proto schemas (dex.sol.v1.*)
  /ingestor/
    geyser/               # Go: Yellowstone gRPC tailer
    helius/               # Go: LaserStream + WS fallback
    common/               # Go: slot->block_time cache, reorg handling
  /decoder/
    raydium/              # Go: program filters; normalize SwapEvent
    orca_whirlpool/       # Go: CLMM fields incl. sqrt_price
    meteora/              # Go: DLMM/CPMM adapter
  /state/
    candle_cpp/           # C++20: candle + pair/pool VWAP builder
    price_cpp/            # C++20: price math, fixed-point utils
  /sinks/
    nats/                 # Go: publishers; NATS subject helpers
    clickhouse/           # Go: batch writers (trades, snapshots, candles)
    parquet/              # Go: s3/minio writers (cold rolls)
  /backfill/
    substreams/           # Rust Substreams modules + manifests
    orchestrator/         # Go: range scheduler; invokes sinks
  /api/
    http/                 # Go: GraphQL/REST read-only
    grpc/                 # Go: internal gRPC for bulk/low-latency
  /bridge/                # Go: TEMP compat bridge to legacy subjects
  /ops/
    jetstream/            # Stream & consumer JSON; nats CLI scripts
    clickhouse/           # DDL .sql files; migrations
    dashboards/           # Grafana JSON; alert rules
  flake.nix / default.nix # dev envs
  Makefile                # builds, proto-gen, fmt, test, run
```

---

## 3. Canonical IDs & Normalization
- `ChainId`: `501`
- `MintId`: base58 string (preserve case)
- `PoolId`: base58 program account (Raydium pool acc; Orca whirlpool; Meteora pool)
- `PairId` canonical rules:
  1. If USDC present → base = other, quote = USDC
  2. Else USDT present → quote = USDT
  3. Else SOL present → quote = SOL
  4. Else lexicographically smaller mint = base
- Token amounts stored as integers (SPL u64) in smallest units; decimals derived from mint account.
- Idempotency key for program logs: `(slot, signature, instruction_index or log_index)`.
- `Nats-Msg-Id = "501:<slot>:<sig>:<index>"` for dedupe across all publishers.

---

## 4. Protobuf Contracts (`/proto/dex/sol/v1/*.proto`)
```proto
syntax = "proto3";
package dex.sol.v1;

message U128 { uint64 hi = 1; uint64 lo = 2; }

message BlockHead {
  uint64 chain_id = 1;      // 501
  uint64 slot     = 2;
  uint64 ts_sec   = 3;      // unix seconds (cluster time)
  string status   = 4;      // "processed" | "confirmed" | "finalized"
}

message TxMeta {
  uint64 chain_id = 1;
  uint64 slot     = 2;
  string sig      = 3;
  bool   success  = 4;
  uint64 cu_used  = 5;
  uint64 cu_price = 6;      // micro-lamports per CU if available
  repeated string log_msgs = 7;
}

message SwapEvent {
  uint64 chain_id   = 1;
  uint64 slot       = 2;
  string sig        = 3;
  uint32 index      = 4;     // instruction/log index
  string program_id = 5;     // raydium/orca/meteora program
  string pool_id    = 6;
  string mint_base  = 7;
  string mint_quote = 8;
  uint32 dec_base   = 9;
  uint32 dec_quote  = 10;
  uint64 base_in    = 11;
  uint64 base_out   = 12;
  uint64 quote_in   = 13;
  uint64 quote_out  = 14;
  uint64 sqrt_price_q64_pre  = 15;
  uint64 sqrt_price_q64_post = 16;
  uint64 reserves_base = 17;
  uint64 reserves_quote= 18;
  uint32 fee_bps       = 19;
  bool   provisional   = 20;
  bool   is_undo       = 21;
}

message PoolSnapshot {
  uint64 chain_id = 1;
  uint64 slot     = 2;
  string pool_id  = 3;
  string mint_base  = 4;
  string mint_quote = 5;
  uint64 sqrt_price_q64 = 6;
  uint64 reserves_base   = 7;
  uint64 reserves_quote  = 8;
  uint32 fee_bps         = 9;
  uint64 liquidity       = 10;
}

message Candle {
  uint64 chain_id     = 1;
  string pair_id      = 2;
  string pool_id      = 3;   // empty for pair-level
  string timeframe    = 4;   // "1s","1m","5m","1h","1d"
  uint64 window_start = 5;
  bool   provisional  = 6;
  bool   is_correction= 7;
  int64 open_px_q32  = 10;
  int64 high_px_q32  = 11;
  int64 low_px_q32   = 12;
  int64 close_px_q32 = 13;
  U128  vwap_num     = 14;
  U128  vwap_den     = 15;
  U128  vol_base     = 16;
  U128  vol_quote    = 17;
  uint32 trades      = 18;
}

message WalletHeuristics {
  uint64 chain_id = 1;
  string wallet   = 2;
  uint64 first_seen_slot = 3;
  uint32 swaps_24h = 4;
  uint32 swaps_7d  = 5;
  bool   is_fresh  = 6;
  bool   is_sniper = 7;
  float  bundled_pct = 8;
}
```

---

## 5. NATS JetStream (Streams & Consumers)
`/ops/jetstream/streams.dex.json`
```json
{
  "name": "DEX",
  "description": "Solana DEX raw + state + candles",
  "subjects": [
    "dex.sol.blocks.head",
    "dex.sol.tx.meta",
    "dex.sol.*.swap",
    "dex.sol.pool.snapshot",
    "dex.sol.candle.pool.*",
    "dex.sol.candle.pair.*"
  ],
  "retention": "limits",
  "storage": "file",
  "discard": "old",
  "replicas": 3,
  "max_age": 0,
  "duplicate_window": "2m"
}
```

`/ops/jetstream/consumer.swaps.json`
```json
{
  "stream_name": "DEX",
  "name": "SWAP_FIREHOSE",
  "filter_subject": "dex.sol.*.swap",
  "ack_policy": "explicit",
  "deliver_policy": "new",
  "replay_policy": "instant",
  "max_deliver": 10,
  "max_ack_pending": 50000,
  "sample_freq": "0%"
}
```
CLI:
```bash
nats stream add --config ops/jetstream/streams.dex.json
nats consumer add DEX --config ops/jetstream/consumer.swaps.json
```

---

## 6. Go Ingestors (Yellowstone + Helius)
- **Environment:**
  ```
  SOL_CHAIN_ID=501
  NATS_URL=nats://user:pass@nats:4222
  NATS_STREAM=DEX
  GEYSER_ENDPOINT=... (Chainstack gRPC)
  GEYSER_API_KEY=...
  HELIUS_GRPC=...
  HELIUS_WS=...
  HELIUS_API_KEY=...
  ```
- **Behavior:**
  - Connect to Geyser gRPC; subscribe to blocks/tx/program logs filtered by `/ops/programs.yaml` IDs.
  - Emit `BlockHead`, `TxMeta`, `SwapEvent` with `provisional=true` at confirmed commitment, and re-emit with `provisional=false` at finalized.
  - Maintain `(slot→ts)` cache; on reconnect, replay last 64 slots.
  - Set `Nats-Msg-Id` for dedupe.
  - Helius module mirrors shapes and can take over automatically if Geyser stream drops.

---

## 7. C++ Candle & State Service
- Build: C++20, Clang 17 (or GCC 13), CMake; Nix flake for toolchain.
- Dependencies: `ankerl::unordered_dense`, `moodycamel::ReaderWriterQueue`, absl(optional), protobuf runtime, NATS C client.
- Threads & sharding: default 8 shard workers, hashed by `pool_id`. Each worker uses packed vectors + index map + timing wheel (no linked lists).
- Lateness policy: 1s window waits 2s; 1m waits 30s; ≥1h wait for finalized commitment.
- Hot path per `SwapEvent`:
  1. Compute price in quote/base (CPMM uses reserves mid or trade amounts; CLMM uses sqrt price squared * decimal adjustment).
  2. Update per-timeframe candle; publish provisional `Candle` to NATS.
  3. On watermark tick (from `BlockHead`), finalize overdue windows, emit `provisional=false`, evict.
- Public config `/state/candle_cpp/config.toml`:
  ```toml
timeframes = ["1s","1m","5m","1h","1d"]
lateness_sec = { "1s"=2, "1m"=30, "5m"=120, "1h"=0, "1d"=0 }
pairs = "pool,pair"
```

---

## 8. ClickHouse DDL (`/ops/clickhouse/*.sql`)
```sql
CREATE TABLE IF NOT EXISTS trades (
  chain_id      UInt16,
  slot          UInt64,
  ts            DateTime64(3, 'UTC'),
  sig           String,
  idx           UInt32,
  program_id    LowCardinality(String),
  pool_id       String,
  mint_base     String,
  mint_quote    String,
  dec_base      UInt8,
  dec_quote     UInt8,
  base_in       Decimal(38,0),
  base_out      Decimal(38,0),
  quote_in      Decimal(38,0),
  quote_out     Decimal(38,0),
  price_q32     Int64,
  provisional   UInt8
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (chain_id, pool_id, slot, sig, idx);
```
```sql
CREATE TABLE IF NOT EXISTS pool_snapshots (
  chain_id    UInt16,
  slot        UInt64,
  ts          DateTime64(3, 'UTC'),
  pool_id     String,
  mint_base   String,
  mint_quote  String,
  sqrt_q64    UInt64,
  reserves_base  Decimal(38,0),
  reserves_quote Decimal(38,0),
  fee_bps     UInt16,
  liquidity   Decimal(38,0)
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (chain_id, pool_id, slot);
```
```sql
CREATE TABLE IF NOT EXISTS ohlcv_1m (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38,0),
  vwap_den     Decimal(38,0),
  vol_base     Decimal(38,0),
  vol_quote    Decimal(38,0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);
```
Duplicate table for 1s/5m/1h/1d. Wallet tables:
```sql
CREATE TABLE IF NOT EXISTS wallet_activity (
  chain_id UInt16, wallet String, first_seen_slot UInt64,
  swaps_24h UInt32, swaps_7d UInt32, is_fresh UInt8, is_sniper UInt8,
  updated_at DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, wallet);

CREATE TABLE IF NOT EXISTS holders_estimate (
  chain_id UInt16, mint String, window_start DateTime('UTC'),
  holders UInt64, method LowCardinality(String),
  updated_at DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, mint, window_start);
```

---

## 9. Substreams Backfill Scaffold
`/backfill/substreams/substreams.yaml`
```yaml
package: "dex_solana_v1"
version: v0.1.0
imports:
  - name: solana
    module: sf.solana.type.v1
modules:
  - name: map_swaps
    kind: map
    inputs:
      - source: sf.solana.type.v1.Block
    output:
      type: proto:dex.sol.v1.SwapEvent
  - name: store_trades
    kind: store
    updatePolicy: add
    valueType: proto:dex.sol.v1.SwapEvent
    inputs:
      - map: map_swaps
  - name: map_pool_snapshots
    kind: map
    inputs:
      - source: sf.solana.type.v1.Block
    output:
      type: proto:dex.sol.v1.PoolSnapshot
```
Example ClickHouse sink:
```bash
substreams run -e mainnet.sol.streamingfast.io:443 substreams.yaml map_swaps \
  --start-block <slot_start> --stop-block <slot_end> \
  | substreams-sink-clickhouse \
    --dsn "tcp://<host>:9000?database=dex" \
    --table trades --flush-interval 2s
```

---

## 10. Observability & SLOs
- Metrics (Prometheus):
  - `ingestor_slot_lag{source=geyser|helius}`
  - `publisher_nats_acks_total{subject}`
  - `candle_updates_total{tf, provisional}`
  - `candle_finalize_latency_ms_bucket{tf}`
  - `decode_errors_total{program_id}`
  - `dedup_drops_total`
  - `clickhouse_write_latency_ms_bucket{table}`
  - `backfill_events_per_sec`
- Targets:
  - Provisional publish p95 < 800 ms.
  - Finalized publish p95 < 20 s.
  - Backfill throughput ≥ 1e6 events/hour/node.
  - Decode error rate < 1e-6 per log.
- Dashboards stored under `/ops/dashboards` (Grafana JSON + alert rules).

---

## 11. Security & Config
- All services operate read-only to RPC; no private keys.
- NATS credentials per service; subject ACL `dex.sol.>`.
- Postgres (control plane) users per component. ClickHouse user with INSERT-only to target tables.
- Config via `.env` or Nix profiles; secrets via 1Password/Credstash (team choice).

---

## 12. Cutover Runbook
A. **Bootstrap infra**
```bash
make up
nats stream add --config ops/jetstream/streams.dex.json
clickhouse-client --queries-file ops/clickhouse/all.sql
```
B. **Dark launch**
```bash
make run.ingestor
make run.candles
make run.sinks
make run.bridge
```
C. **Shadow compare (7 days)**
- Compare per-pool OHLCV & trade counts vs legacy WS aggregates (alert on |Δ| > thresholds).
- Monitor JetStream consumer lag & replay counters; avoid redelivery storms.
D. **Switch consumers**
- Point internal services to `dex.sol.candle.*` firehose or new GraphQL/HTTP.
- Leave bridge running for one week.
E. **Retire legacy**
- Stop legacy Rust market-data service; delete bridge; prune old subjects.

---

## 13. Tests & Fixtures (Must Pass)
- Golden tests: tx→swap conversions, sqrt→price conversions (CLMM), CPMM reserves mid-price, decimals scaling.
- Replay test: 1h slot range across Raydium & Whirlpool; assert `|ΔOHLC| ≤ 0.1%`, `|Δvolume| ≤ 0.1%`, `|Δtrades| ≤ 0.1%`.
- Watermark test: simulate late events before/after finalization; ensure provisional→final transitions & corrections.
- Reorg test: inject undo events < finalization depth; candles recompute identically.
- Perf: benchmark shard worker at 100k events/s (8 shards) with P95 update < 800 ms.

---

## 14. Work Breakdown (Day0 Parallelizable)
- **Contracts & infra**
  - [ ] Author `.proto` files; `make proto-gen` for Go & C++
  - [ ] NATS stream/consumer JSON + scripts
  - [ ] ClickHouse DDL; seed program registry
- **Ingestors (Go)**
  - [ ] Geyser ingestor: block/tx/program filters; slot/ts cache; provisional/final publish; reorg undo
  - [ ] Helius ingestor: WS/gRPC fallback; same message shapes; automatic takeover
  - [ ] decoder/*: per-program adapters to `SwapEvent`
- **Candle & state (C++20)**
  - [ ] Fixed-point utils (q32.32), u128 helpers
  - [ ] Packed store + index + timing wheel
  - [ ] CPMM/CLMM/DLMM price adapters
  - [ ] Per-tf window update + finalize + correction path
  - [ ] NATS publisher; ClickHouse/Parquet sink integration hooks
- **Sinks (Go)**
  - [ ] ClickHouse batch writer with upsert (ReplacingMergeTree)
  - [ ] Parquet roll writer (hourly/daily) to S3/MinIO
- **Backfill (Substreams + Orchestrator)**
  - [ ] `map_swaps`, `map_pool_snapshots`; Raydium/Orca/Meteora coverage
  - [ ] ClickHouse sink wiring; orchestrator range scheduler
- **API (Go)**
  - [ ] GraphQL/REST: latest price, pool snapshot, candles, wallet stats
  - [ ] Pagination; rate limits
- **Bridge (Go, TEMP)**
  - [ ] `dex.sol.*` → legacy subjects; reuse idempotency keys
- **Observability**
  - [ ] Prom metrics in each service; Grafana dashboards; alerts

---

## 15. Config & Reference Files
`/ops/programs.yaml`
```yaml
raydium:
  amm_program: "RVKd61ztZW9..."
orca_whirlpools:
  program: "whirLbB1kuT..."
meteora:
  program: "METoRa1111..."
stables:
  usdc: "EPjFWdd5Aufq..."
  usdt: "Es9vMFrzaCER..."
wrapped_sol:
  wsol: "So11111111111111111111111111111111111111112"
```
`.env.example`
```
NATS_URL=nats://nats:4222
CLICKHOUSE_DSN=tcp://clickhouse:9000?database=dex
S3_ENDPOINT=http://minio:9000
S3_BUCKET=dex-parquet
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
GEYSER_ENDPOINT=...
GEYSER_API_KEY=...
HELIUS_GRPC=...
HELIUS_WS=...
HELIUS_API_KEY=...
```

---

## 16. Implementation Notes & Guarantees
- Exactly-once downstream via `(slot, sig, index)` keys; JetStream dedupe + ClickHouse PK prevents double count.
- Late events accepted until finalization; afterwards emit `is_correction=true` rather than silent edits.
- USDC anchor pegged to $1 unless |px−1| ≥ 0.5% for ≥ 60s; when depegged route via SOL/other stables; flag `usdc_depeg=1`.
- Publish both pool-level and pair-level candles; pair VWAP is quote-volume weighted across pools.
- Prices stored as q32.32; create ClickHouse view to expose Decimal(38,9) for analytics.

---

## 17. “Own Your Pipeline” Notes (Lean Budget Variant)
- Prefer Yellowstone stream filtering to Raydium CLMM + Orca Whirlpools accounts.
- Skip full-chain Firehose storage (large; ~61 GiB/day compressed).
- Chainstack Geyser stream ≈ $49/mo (sufficient when filtering).
- Optional Pyth feed for variance checks.
- Canvas/WebGL frontend fed by typed arrays; render 100k bars @ 60fps with incremental updates.

(See `docs/OVERVIEW.md` for the full narrative.)

---
