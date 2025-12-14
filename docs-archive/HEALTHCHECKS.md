# Health & Readiness Checks

## Go Services (ingestors, sinks, bridge, API)
- `/healthz` – simple liveness (process up).
- `/readyz` *(todo)* – verify downstream dependencies (JetStream connection, Redis/ClickHouse connectivity, slot lag thresholds).

## Candle Engine (C++)
- Expose gRPC/HTTP endpoint reporting shard queue depth, last watermark, finalize lag.
- Return non-ready if backlog exceeds configured threshold or finalizer thread not running.

## Backfill Orchestrator
- `/status` endpoint summarizing active ranges, checkpoint progress, worker health.

## JetStream Verification
- `make ops.jetstream.verify` ensures `DEX` stream + `SWAP_FIREHOSE` consumer exist.
- `scripts/jetstream-validate.sh` prints retention, duplicates, replicas, ack stats; requires `jq` and `nats` CLI.

## Alerts
- Slot lag > 64 for >5 minutes.
- Consumer ack pending > 50k.
- Candle finalize lag > configured lateness (e.g., >30 s for 1 m windows).
