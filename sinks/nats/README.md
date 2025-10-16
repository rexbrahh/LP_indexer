# NATS JetStream Publisher

This package implements the JetStream publishers described in the Solana
liquidity indexer specification. It accepts canonical `dex.sol.v1` protobuf
messages and publishes them to the subjects defined in
`ops/jetstream/streams.dex.json`, setting the `Nats-Msg-Id` header for
exactly-once semantics.

## Responsibilities

* Maintain a single JetStream connection with sensible defaults.
* Publish the following subjects (at minimum):
  - `dex.sol.blocks.head`
  - `dex.sol.tx.meta`
  - `dex.sol.*.swap`
  - `dex.sol.pool.snapshot`
  - `dex.sol.candle.pool.*`
  - `dex.sol.candle.pair.*`
* Encode protobuf payloads, set `Content-Type`, and attach deduplication
  headers (`Nats-Msg-Id`).
* Subject naming handles Raydium/Orca/Meteora program IDs, with safe fallbacks
  for unknown programs.

## Configuration

The environment variables mirror the spec:

| Variable            | Description                                  |
| ------------------- | -------------------------------------------- |
| `NATS_URL`          | Connection URL (e.g. `nats://user:pass@host`).|
| `NATS_STREAM`       | Stream name (`DEX`).                          |
| `NATS_SUBJECT_ROOT` | Optional subject prefix override.            |
| `NATS_PUBLISH_TIMEOUT_MS` | Publish timeout (default 5000ms).      |

See `config.go` for full details.

## Next Steps

1. Expose Prometheus metrics (`publisher_nats_acks_total`,
   `publisher_nats_errors_total`, etc.).
2. Add optional TLS/creds handling for production deployments.
3. Integrate with the ingestor failover controller for health-aware publishing.
