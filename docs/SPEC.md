# Solana Liquidity Indexer – Product Specification (v0.1)

## 1. Scope
- **Chain:** Solana mainnet-beta (`chain_id = 501`)
- **Programs (phase 1):** Raydium CLMM, Orca Whirlpools, Meteora DLMM/CPMM
- **Commitments:** publish provisional data at `confirmed`, finalize at `finalized`
- **Timeframes:** 1s, 1m, 5m, 1h, 1d
- **Languages:** Go for streaming/sinks/APIs; C++20 for candle and price compute

## 2. Data Contracts
- Transport: NATS JetStream (`DEX` stream)
- Dedup: `Nats-Msg-Id = "501:<slot>:<sig>:<index>"`
- Protobuf package `dex.sol.v1` exposes: `BlockHead`, `TxMeta`, `SwapEvent`, `PoolSnapshot`, `Candle`, `WalletHeuristics`, `U128`
- All token amounts stored as integer base units (SPL u64)

## 3. Realtime ingest
| Source | Purpose | Notes |
|--------|---------|-------|
| Yellowstone Geyser gRPC (Chainstack) | Primary tip stream | Replay last 64 slots on reconnect |
| Helius LaserStream/WS | Automatic fallback | Mirror message shapes, dedupe via Msg-Id |
| Substreams (StreamingFast) | Backfill + heavy derivations | Slot-range playback into ClickHouse & Parquet |

## 4. Normalization rules
1. If USDC present → treat as quote token (mint stays canonical)
2. Else USDT, else SOL, else lexicographic order
3. Pair IDs formatted `base_mint:quote_mint`
4. `swap.amount_*` remain raw ints; decimals metadata carried on the message

## 5. Outputs
- `dex.sol.blocks.head` – block metadata, slot timestamps
- `dex.sol.tx.meta` – transaction success, CU usage, logs
- `dex.sol.<program>.swap` – canonical swap events with provisional flag
- `dex.sol.pool.snapshot` – CLMM/DLMM state (sqrt price, reserves, fees)
- `dex.sol.candle.pool.*` / `dex.sol.candle.pair.*` – OHLCV per pool/pair per timeframe
- `dex.sol.wallet.heuristics` – early wallet classification

## 6. Storage
- ClickHouse MergeTree tables for trades, pool snapshots, per-timeframe OHLCV, wallet activity
- Parquet cold storage (MinIO/S3) rolled hourly/daily for replay/audit
- JetStream retention `limits`, storage `file`, replicas 3, duplicate window 2 minutes

## 7. Compute guarantees
- Idempotent publish via Msg-Id header
- Candle engine uses Q32.32 fixed-point, 128-bit accumulators, shards by `pool_id`
- 1s timing wheel finalizes windows using watermarks from block heads

## 8. Observability SLOs
- Provisional publish p95 < 800 ms
- Finalized publish p95 < 20 s
- Backfill throughput ≥ 1e6 swaps/hour/node
- Decoder error rate < 1e-6/log, dedup drop rate < 1e-4

## 9. Phase plan
1. **P0 Foundations:** proto + tooling baseline, CI, docs
2. **P1 Realtime ingest:** Yellowstone + Helius with Raydium/Orca/Meteora decoders
3. **P2 State compute:** C++ candle engine emitting to JetStream & ClickHouse
4. **P3 Backfill parity:** Substreams replay, compare vs legacy WS aggregates
5. **P4 API & bridge:** HTTP/GraphQL, JetStream bridge to legacy stack
6. **P5 Legacy retirement:** turn off existing Rust market-data, keep new firehose

## 10. Open items
- Persist slot checkpoints (Redis/Postgres) for crash recovery
- Prometheus metrics exporter for ingestors and candle service
- Upgrade path for long-term storage (NATS⇄Kafka bridge evaluation)
