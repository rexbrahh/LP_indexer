# Operational Runbooks

## JetStream
- Init: `make ops.jetstream.init`
- Verify: `make ops.jetstream.verify`
- Detailed report: `./scripts/jetstream-validate.sh`

## Bridge
- Run locally: `make run.bridge`
- Check Prometheus metrics: `make check.bridge.metrics` (expects the bridge metrics endpoint to be live)
- Subject mappings live at `ops/bridge/subject_map.yaml`

## Geyser Ingestor
- Run locally: `make run.ingestor.geyser`
- Demo streaming harness: `GEYSER_ENDPOINT=... GEYSER_API_KEY=... make demo.geyser`
- Configure program filters via `PROGRAMS_YAML_PATH` (default `ops/programs.yaml`).
- Set `NATS_URL` / `NATS_STREAM` / `NATS_SUBJECT_ROOT` to control JetStream publishing.
- Metrics (if enabled) exposed at `INGESTOR_METRICS_ADDR` (default `:9101`).
- Helios fallback: set `ENABLE_HELIUS_FALLBACK=1` with `HELIUS_GRPC`, `HELIUS_WS`,
  and `HELIUS_API_KEY` configured; the ingestor will fail over automatically if
  the primary Geyser stream errors.

## ClickHouse
- Apply schema: `make ops.clickhouse.apply` (respects `CLICKHOUSE_DSN`, falls back to docker exec when local client is missing).
- Monitor write latency via exported Prometheus metrics (`clickhouse_write_latency_ms_bucket`).

## Backfill
- Run Substreams command (see `docs/BACKFILL.md`).
- Track checkpoints and throughput; ensure sink keeps up before widening slot ranges.

## Cutover
- Follow the detailed steps in `docs/CUTOVER.md` (bootstrap → dark launch → shadow compare → flip → retire).
- Before switching consumers, confirm `make ops.jetstream.verify` succeeds and parity metrics are within tolerance.

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
