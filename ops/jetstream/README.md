# JetStream Operations Guide

JetStream backs the real-time event bus for the Solana Liquidity Indexer. All data products share a single stream `DEX` with well-defined subjects and a strict deduplication policy.

## Stream Layout

`ops/jetstream/streams.dex.json`

```
subjects:
  - dex.sol.blocks.head
  - dex.sol.tx.meta
  - dex.sol.*.swap
  - dex.sol.pool.snapshot
  - dex.sol.candle.pool.*
  - dex.sol.candle.pair.*
retention: limits
storage: file
replicas: 3
duplicate_window: 2m
max_age: 0
```

All publishers must set `Nats-Msg-Id = "501:<slot>:<sig>:<index>"` to preserve exactly-once semantics.

## Consumers

`ops/jetstream/consumer.swaps.json` defines the canonical durable consumer for swap events:

```
name: SWAP_FIREHOSE
filter_subject: dex.sol.*.swap
ack_policy: explicit
deliver_policy: new
replay_policy: instant
max_ack_pending: 50000
max_deliver: 10
```

Downstream services (candles, sinks, bridge) should reuse this consumer or register their own durable consumers with explicit acknowledgements.

## Prerequisites

Install the NATS CLI:

```bash
brew install nats-io/nats-tools/nats        # macOS
# or
curl -sf https://binaries.nats.dev/nats-io/natscli/nats@latest | sh
```

Configure a context before issuing commands:

```bash
nats context save local --server=localhost:4222
nats context select local
```

## Quick Start

Initialise the stream and default consumer:

```bash
make ops.jetstream.init
```

This target executes:

```bash
nats stream add --config ops/jetstream/streams.dex.json
nats consumer add DEX --config ops/jetstream/consumer.swaps.json
```

## Operational Commands

### Streams

```bash
nats stream ls                # list streams
nats stream info DEX          # detailed stream information
nats stream report usage      # cluster usage snapshot
nats stream edit DEX --replicas=3
nats stream purge DEX --subject "dex.sol.candle.pool.1m"   # purge specific subject (use cautiously)
```

### Consumers

```bash
nats consumer ls DEX
nats consumer info DEX SWAP_FIREHOSE
nats consumer add DEX my-worker \
  --pull \
  --filter="dex.sol.candle.pool.*" \
  --ack=explicit \
  --max-deliver=5 \
  --replay=instant
nats consumer rm DEX my-worker
```

### Publishing & Subscription

```bash
nats pub dex.sol.raydium.swap \
  --header="Msg-Id:501:12345678:abc123:0" \
  '{"slot":12345678,"signature":"abc123","pool":"raydium_xyz","amount_in":1000}'

nats sub dex.sol.pool.snapshot
```

### Monitoring

Recommended JetStream metrics:

- `jetstream_stream_state_messages` – message volume per stream
- `jetstream_consumer_num_ack_pending` – backlog per consumer
- `jetstream_consumer_num_redelivered` – redelivery rate (watch for persistent retries)
- `jetstream_stream_state_bytes` – disk usage (aligns with retention policy)

Use `nats top` or your preferred Prometheus/Grafana dashboards for live observability.

## Runbook Tips

- **Backpressure:** rising `ack_pending` with flat publish rate usually indicates a stalled consumer. Investigate before extending `max_ack_pending`.
- **Replays:** when replaying historical ranges, advance the dedupe window if gaps exceed `2m`.
- **Schema changes:** introduce new subjects (e.g., `dex.sol.wallet.heuristics`) by updating `streams.dex.json` and re-running `make ops.jetstream.init`.
