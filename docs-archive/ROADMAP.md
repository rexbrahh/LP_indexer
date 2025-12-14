# Roadmap

## Milestone 1 – Foundations
- Proto contracts stubbed, tooling (`make lint`, `go test`, `scripts/build_candles.sh`) stable.
- Geyser demo connects with token auth, slot cache documented.

## Milestone 2 – Realtime Pipeline
- Yellowstone + Helius ingestors publishing provisional swap events.
- Raydium & Orca decoders canonicalizing data; Meteora added next.

## Milestone 3 – State Compute
- Candle engine finalizes all timeframes, publishes to NATS & ClickHouse.
- Wallet heuristics and price maths implemented in C++ modules.

## Milestone 4 – Backfill & Parity
- Substreams backfill for 90 days; parity checks vs legacy feed automated.
- ClickHouse/Parquet sinks tested end-to-end.

## Milestone 5 – APIs & Observability
- HTTP/GraphQL API with Redis cache ready for consumers; gRPC for internal consumers.
- Prometheus/Grafana dashboards with SLO alerting.

## Milestone 6 – Cutover & Legacy Retirement
- Shadow run complete, downstream consumers migrated.
- Legacy Rust market-data service decommissioned and repo cleaned up.

Stretch goals: Kafka bridge for long retention, additional DEX coverage, portfolio analytics APIs.
