# NATS JetStream Publisher (Scaffold)

This package contains the scaffolding for the JetStream publishers described in
the Solana liquidity indexer specification. The eventual implementation will
take canonical `dex.sol.v1` protobuf messages and publish them to the JetStream
subjects defined in `ops/jetstream/streams.dex.json` with exactly-once semantics
(`Nats-Msg-Id = "501:<slot>:<sig>:<index>"`).

## Responsibilities

* Maintain a single JetStream connection with sensible reconnect behaviour.
* Publish the following subjects (at minimum):
  - `dex.sol.blocks.head`
  - `dex.sol.tx.meta`
  - `dex.sol.*.swap`
  - `dex.sol.pool.snapshot`
  - `dex.sol.candle.pool.*`
  - `dex.sol.candle.pair.*`
* Encode protobuf payloads, set `Content-Type`, and attach deduplication
  headers.
* Surface Prometheus metrics (`publisher_nats_acks_total`,
  `publisher_nats_errors_total`, etc.).

At present the module only exposes configuration parsing, validation, and a
`Publisher` type that returns `ErrNotImplemented`. Integrations can depend on
the API immediately while the JetStream wiring lands in a follow-up change.

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

1. Implement JetStream connection management using `github.com/nats-io/nats.go`.
2. Add typed `PublishSwap`, `PublishCandle`, etc. that marshal protobufs and set
   `Nats-Msg-Id`.
3. Expose health metrics and integrate with the failover logic in the ingestors.
4. Add integration tests targeting a Dockerised NATS JetStream instance.
