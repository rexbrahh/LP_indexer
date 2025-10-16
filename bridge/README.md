# Legacy Bridge (Scaffold)

Temporary compatibility service that mirrors the new `dex.sol.*` subjects to the
legacy market-data topics used by downstream systems. The locked spec requires
this bridge during dark launch so that old consumers continue working while we
migrate them to the new contracts.

## Responsibilities

* Subscribe to the canonical JetStream subjects (swaps, candles, snapshots).
* Transform payloads into legacy subjects using a configurable prefix mapping.
* Preserve idempotency using the same `Nats-Msg-Id = "501:<slot>:<sig>:<index>"`.
* Expose health metrics and lag monitors. _(TODO)_

## Configuration

Environment variables:

| Variable | Description |
| --- | --- |
| `BRIDGE_SOURCE_NATS_URL` | JetStream cluster that hosts the canonical `dex.sol.*` subjects. |
| `BRIDGE_TARGET_NATS_URL` | Legacy NATS cluster to publish mirrored messages. |
| `BRIDGE_SOURCE_STREAM` | Source stream name (e.g. `DEX`). |
| `BRIDGE_TARGET_STREAM` | Target stream name (e.g. `legacy`). |
| `BRIDGE_SUBJECT_MAP_PATH` | Optional YAML file describing subject prefix translations. |
| `BRIDGE_METRICS_ADDR` | Optional address (e.g. `:9090`) for the Prometheus `/metrics` endpoint. |

Subject mapping file example:

```yaml
mappings:
  - source: "dex.sol.swap."
    target: "legacy.swap."
  - source: "dex.sol.debug"
    drop: true
```

Rules are evaluated from longest to shortest source prefix. If `drop: true` the
message is acknowledged but not forwarded. When no mapping matches the subject
the bridge falls back to publishing the original subject unchanged.
