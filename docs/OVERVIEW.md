# System Overview & Lean Pipeline Notes

## Why build our own pipe?
"Why pay when we can spend 10× longer to ship 10× worse?" – the goal is to own the Solana market-data stack end to end, tailored for Raydium CLMM + Orca Whirlpools (with Meteora following) while keeping runtime costs manageable.

## End-to-end shape
```
Yellowstone Geyser (Chainstack) ──► NATS JetStream ──► Rust/Go decoders
                                            │
                                            └─► ClickHouse {trades, pools, bars_1s, bars_1m}
                                                     │
                                                     └─► API (Go) + Redis hot window
                                                             │
                                                             └─► WebGL/Canvas chart engine
```

## Cost-conscious choices
- **Stream, don’t poll RPC:** Chainstack Geyser stream (~$49/mo) filtered to Raydium/Orca accounts is sufficient for start.
- **Avoid full Firehose storage:** Solana Firehose merged blocks ≈61 GiB/day compressed—overkill until analytics demand it.
- **Helius:** reserve for later when webhooks/high RPS needed; pricing scales with usage.
- **ClickHouse** + Parquet for analytics; keep on NVMe, roll to S3/MinIO for cold storage.

## Data to ingest
- **Programs:**
  - Raydium CLMM (open-source types help decoding).
  - Orca Whirlpools (CLMM using Q64.64 sqrt price).
- **Event types:** swaps, liquidity add/remove, pool state updates.
- **Sanity feed:** Pyth price stream optional for variance flagging.

## CLMM price math refresher
- Swap price derived from sqrt price in Q64.64 fixed point: `p = (sqrt/2^64)^2 * 10^(dec_base - dec_quote)`.
- Volumes stored in smallest units; convert using mint decimals only at presentation time.

## Aggregation
- Bucket swaps into 1-second bars (carry forward to longer intervals). First trade sets open/high/low/close; subsequent trades update high/low/close and accumulate volume/trade count.
- Carry-forward to 1 minute / 5 minute / 1 hour / 1 day using deterministic window start floor.

## Storage model
- `trades` (MergeTree, PK `(slot,sig,index)`).
- `pools` (snapshots, ReplacingMergeTree).
- `bars_1s` (SummingMergeTree) with materialized view to 1m bars.
- Additional tables for wallet heuristics & holders estimates.

## Streaming and backfill
- **Realtime:** subscribe to Yellowstone accounts + program logs, produce normalized protobufs into JetStream.
- **Backfill:** run Substreams (`map_swaps`, `map_pool_snapshots`) piping output to ClickHouse or Parquet. End result identical to live math.

## Front-end strategy
- Serve typed arrays over WebSocket/HTTP for thin clients; use Canvas/WebGL (100k bars @ 60 fps). Update only latest candle per delta.

## Acceptance targets (lean budget)
- Provisional latency < 800 ms; finalized < 20 s.
- Backfill throughput ≥ 1e6 events/hour/node.
- Data parity vs legacy feed: |ΔOHLC| ≤ 1% (or 1 CLMM tick), |Δvolume| ≤ 0.1%, |Δtrades| ≤ 0.1%.

## Next steps when scaling up
- Kafka bridge for long retention/partitioning.
- Additional DEX venues (Lifinity, Saber, etc.).
- Advanced analytics (portfolio stats, wallet classifications).
