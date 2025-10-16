# Operational Runbooks

## JetStream
- Init: `make ops.jetstream.init`
- Verify: `make ops.jetstream.verify`
- Detailed report: `./scripts/jetstream-validate.sh`

## ClickHouse
- Apply schema (once DDL is committed): `clickhouse-client --queries-file ops/clickhouse/all.sql`
- Monitor write latency via exported Prometheus metrics (`clickhouse_write_latency_ms_bucket`).

## Backfill
- Run Substreams command (see `docs/BACKFILL.md`).
- Track checkpoints and throughput; ensure sink keeps up before widening slot ranges.

## Alerting
- JetStream lag (`ack_pending`, `num_pending`), decode errors, candle finalize latency.
- Use Grafana dashboards under `ops/dashboards/` (to be populated).

## Incident Response
1. Check JetStream consumer lag / redeliveries.
2. Inspect decoder logs for parse errors or normalization drift.
3. Validate slot cache timestamps; replay last 64 slots if necessary.
4. Coordinate with RPC provider if head slot stalls.

## Maintenance Tasks
- Rotate API tokens (Chainstack/Helius) and update secret store.
- Vacuum ClickHouse partitions older than retention window.
- Test failover path (Helius fallback) quarterly.
