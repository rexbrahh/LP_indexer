# Operational Runbooks

## JetStream
- Init: `make ops.jetstream.init`
- Verify: `make ops.jetstream.verify`
- Detailed report: `./scripts/jetstream-validate.sh`
- Finalization checks: monitor `dex.sol.blocks.head` for `status=confirmed` → `status=finalized` transitions, and ensure `dex.sol.*.swap` replays emit a second (non-`provisional`) message per slot. `status=dead` entries indicate undo publication and should be accompanied by `SwapEvent.is_undo=1`.

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
- Swap finalization: the processor caches provisional swaps and rewrites them when `SlotStatus=FINALIZED` arrives. Expect two messages per swap (provisional, finalized) and an optional third undo message when `SlotStatus=DEAD`.

## ClickHouse
- Apply schema: `make ops.clickhouse.apply` (respects `CLICKHOUSE_DSN`, falls back to docker exec when local client is missing).
- Monitor write latency via exported Prometheus metrics (`clickhouse_write_latency_ms_bucket`).
- New fields: `ops/clickhouse/trades.sql` includes `reserves_base`, `reserves_quote`, `fee_bps`, and `is_undo` columns—apply migrations before starting the sink.

## ClickHouse Sink Service
- Environment:
  - `CH_SINK_NATS_URL`, `CH_SINK_NATS_STREAM`, `CH_SINK_SUBJECT_ROOT` (default `dex.sol`)
  - `CH_SINK_CONSUMER` (JetStream durable name, default `clickhouse-sink`)
  - `CH_SINK_PULL_BATCH`, `CH_SINK_PULL_TIMEOUT_MS`
  - `CH_SINK_DSN`, `CH_SINK_DATABASE`, `CH_SINK_TRADES_TABLE`, `CH_SINK_CANDLES_TABLE`
  - `CH_SINK_BATCH_SIZE`, `CH_SINK_FLUSH_INTERVAL_MS`
  - Optional retry tuning: `CH_SINK_MAX_RETRIES`, `CH_SINK_RETRY_BACKOFF_MS`, `CH_SINK_RETRY_BACKOFF_MAX_MS`
- Run locally: `go run ./cmd/sink/clickhouse` (or wrap in supervisor). The service consumes `dex.sol.blocks.head`, `dex.sol.tx.meta`, and `dex.sol.*.swap`, writing finalized/undo rows to ClickHouse.
- Validation:
  - `SELECT * FROM trades ORDER BY slot DESC LIMIT 5` to spot-check new swaps (expect `provisional` toggling to `0` and `is_undo=1` when applicable).
  - Monitor retry/backoff via service logs; JetStream redeliveries should stay near zero once the sink is healthy.

## Parquet Sink Service
- Environment:
  - `PARQUET_NATS_URL`, `PARQUET_NATS_STREAM`, `PARQUET_SUBJECT_ROOT`
  - `PARQUET_CONSUMER`, `PARQUET_PULL_BATCH`, `PARQUET_PULL_TIMEOUT_MS`
  - S3 config via existing writer variables: `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `PARQUET_PREFIX`, `PARQUET_FLUSH_INTERVAL_S`, `PARQUET_BATCH_ROWS`, `PARQUET_REGION`
- Run locally: `go run ./cmd/sink/parquet`
- Output layout: objects land under `<prefix>/timeframe=<tf>/scope=<pool|pair>/date=YYYY-MM-DD/candles-<unix>.parquet`.
- Validation: download the most recent object and inspect with `parquet-cat` or DuckDB to confirm fields (scope, provisional flag, VWAP numerics) are set.

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
