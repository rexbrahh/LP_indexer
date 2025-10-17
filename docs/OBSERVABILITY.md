# Observability & Metrics

## Prometheus Metrics (target set)
- `ingestor_slot_lag{source=geyser|helius}` – head slot minus last processed slot.
- `publisher_nats_acks_total{subject}` – JetStream ack stats.
- `candle_updates_total{tf, provisional}` – count of provisional/final candles per timeframe.
- `candle_finalize_latency_ms_bucket{tf}` – histogram from provisional to finalized.
- `decode_errors_total{program_id}` – parser/normalization errors.
- `dedup_drops_total` – duplicate messages dropped via Msg-Id.
- `clickhouse_write_latency_ms_bucket{table}` – sink write latency.
- `backfill_events_per_sec` – Substreams throughput.
- `bridge_forward_total{subject}` – messages mirrored to legacy subjects.
- `bridge_dropped_total{subject}` – messages acknowledged but not forwarded.
- `bridge_publish_errors_total{subject}` – legacy publish failures.
- `bridge_source_lag_seconds{subject}` – source stream age observed by the bridge.
- `ingestor_raydium_swaps_total` – count of Raydium swaps decoded from Geyser.
- `ingestor_raydium_decode_errors_total` – decoder or publish failures for Raydium flow.

## Dashboards (planned)
- Ingestor health (slot lag, error rate, reconnect count).
- Candle engine latency (per timeframe) and shard workload.
- JetStream utilisation (messages, bytes, ack pending, redelivery).
- ClickHouse writes & retry counts.

## Logging
- Structured logging (zap/logrus); include fields: `chain_id`, `pool_id`, `pair_id`, `slot`, `sig`.
- Emit watermarks and replay actions at debug level.

## Tracing (future)
- Adopt OpenTelemetry for request traces (ingestor → decoder → candle → sink/API).
- Propagate trace IDs through JetStream metadata where feasible.
