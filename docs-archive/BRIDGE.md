# Legacy Bridge Runbook

The legacy bridge mirrors the canonical `dex.sol.*` JetStream subjects to the
legacy market-data subjects used during cutover. Use it during dark launch and
shadow comparison to keep downstream consumers in sync.

## Configuration

Environment variables:

| Variable | Required | Description |
| --- | --- | --- |
| `BRIDGE_SOURCE_NATS_URL` | ✅ | NATS URL for the canonical stream (e.g. `nats://127.0.0.1:4222`). |
| `BRIDGE_TARGET_NATS_URL` | ✅ | NATS URL for the legacy cluster. Defaults to source URL. |
| `BRIDGE_SOURCE_STREAM` | ✅ | Source stream name (default `DEX`). |
| `BRIDGE_TARGET_STREAM` | ✅ | Target stream name (default `legacy`). |
| `BRIDGE_SUBJECT_MAP_PATH` | ✅ | YAML file describing subject prefix translations. |
| `BRIDGE_METRICS_ADDR` | ❌ | Optional address for Prometheus metrics (e.g. `:9090`). |

Subject map example (`ops/bridge/subject_map.yaml`):

```yaml
mappings:
  - source: "dex.sol.swap.raydium"
    target: "legacy.dex.swap.raydium"
  - source: "dex.sol.swap.orca"
    target: "legacy.dex.swap.orca"
  - source: "dex.sol.swap."
    target: "legacy.dex.swap."
  - source: "dex.sol.candle.pool."
    target: "legacy.dex.candle.pool."
  - source: "dex.sol.candle.pair."
    target: "legacy.dex.candle.pair."
  - source: "dex.sol.pool.snapshot"
    target: "legacy.dex.pool.snapshot"
  - source: "dex.sol.debug"
    drop: true
```

## Running Locally

1. Ensure NATS (with JetStream) is running and `make ops.jetstream.init` has been
   applied.
2. Start the bridge using the provided Make target:

   ```bash
   make run.bridge
   ```

   Override any env values inline, for example:

   ```bash
   BRIDGE_TARGET_NATS_URL=nats://legacy:4222 BRIDGE_METRICS_ADDR=:9100 make run.bridge
   ```

3. Visit `http://localhost:9090/metrics` (or chosen port) to inspect the
   exported Prometheus metrics (`dex_bridge_forward_total`, `dex_bridge_dropped_total`, etc.).
4. Optionally assert the metrics endpoint responds using `make check.bridge.metrics`
   (override `BRIDGE_METRICS_URL` if the port differs).

## Metrics

The bridge exports the following Prometheus series:

- `dex_bridge_forward_total{subject}` – mirrored messages.
- `dex_bridge_dropped_total{subject}` – drops due to mapping rules.
- `dex_bridge_publish_errors_total{subject}` – publish failures.
- `dex_bridge_source_lag_seconds{subject}` – observed age of source messages.

Process and Go runtime collectors are registered automatically.

## Shutdown

The binary handles SIGINT/SIGTERM and exits cleanly, closing NATS connections
and stopping the metrics server.
