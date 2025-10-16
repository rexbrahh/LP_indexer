# Project Overview

Solana Liquidity Indexer ingests on-chain activity (Raydium, Orca Whirlpools, Meteora) directly from the network, normalizes swap/state events, computes candles in C++, and exposes canonical data via NATS, ClickHouse, Parquet, and HTTP/GraphQL APIs.

## High-Level Flow
```
Yellowstone/Helius -> Ingestor -> Decoder -> Candle Engine
                                       |            |
                                       v            v
                                   JetStream     ClickHouse/Parquet
                                        |
                                        v
                                   HTTP/gRPC APIs
```

## Key Goals
- Deliver near real-time swaps, pool snapshots, and multi-timeframe OHLCV with deterministic math.
- Provide exactly-once semantics downstream (Msg-Id dedup, ReplacingMergeTree upserts).
- Backfill historical ranges via Substreams, ensuring parity with legacy market-data service.
- Ensure clear cutover runbook and observability for operational confidence.
