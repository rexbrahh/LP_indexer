# JetStream Operations Guide

This directory contains operational configuration and documentation for NATS JetStream streams and consumers used by the Solana Liquidity Indexer.

## Overview

The indexer uses NATS JetStream for exactly-once message delivery with the following subjects:
- `dex.sol.swaps` – Raw swap events from all DEX programs
- `dex.sol.candles.1m` – 1-minute OHLCV candles
- `dex.sol.candles.5m` – 5-minute OHLCV candles
- `dex.sol.candles.1h` – 1-hour OHLCV candles

## Prerequisites

Install the NATS CLI:
```bash
# macOS
brew install nats-io/nats-tools/nats

# Linux
curl -sf https://binaries.nats.dev/nats-io/natscli/nats@latest | sh

# Or via Go
go install github.com/nats-io/natscli/nats@latest
```

## Quick Start

Initialize all streams and consumers:
```bash
make ops.jetstream.init
```

This will create the necessary streams with the following configuration:
- **Retention**: Work queue (exactly-once semantics)
- **Deduplication**: 1-hour window based on `Msg-Id` header
- **Storage**: File-based for durability
- **Replicas**: 3 (for production clusters)

## NATS CLI Commands

### Connection

Set your NATS server context (required before other commands):
```bash
# Local development
nats context save local --server=localhost:4222

# Production (with credentials)
nats context save prod \
  --server=nats://prod-cluster:4222 \
  --creds=/path/to/creds.nats

# Select active context
nats context select local
```

### Stream Management

#### Create Streams

Create the main swap events stream:
```bash
nats stream add dex-swaps \
  --subjects="dex.sol.swaps" \
  --storage=file \
  --retention=workq \
  --replicas=3 \
  --discard=old \
  --max-age=168h \
  --max-msg-size=1048576 \
  --dupe-window=1h
```

Create candle streams:
```bash
# 1-minute candles
nats stream add dex-candles-1m \
  --subjects="dex.sol.candles.1m" \
  --storage=file \
  --retention=workq \
  --replicas=3 \
  --max-age=720h \
  --dupe-window=1h

# 5-minute candles
nats stream add dex-candles-5m \
  --subjects="dex.sol.candles.5m" \
  --storage=file \
  --retention=workq \
  --replicas=3 \
  --max-age=2160h \
  --dupe-window=1h

# 1-hour candles
nats stream add dex-candles-1h \
  --subjects="dex.sol.candles.1h" \
  --storage=file \
  --retention=workq \
  --replicas=3 \
  --max-age=8760h \
  --dupe-window=1h
```

#### List Streams

```bash
# List all streams
nats stream ls

# View stream details
nats stream info dex-swaps

# View stream state (messages, bytes, consumers)
nats stream state dex-swaps
```

#### Update Streams

```bash
# Update max age
nats stream edit dex-swaps --max-age=336h

# Update replicas (requires cluster)
nats stream edit dex-swaps --replicas=3
```

#### Delete Streams

```bash
# Delete a stream (WARNING: destroys all messages)
nats stream rm dex-swaps

# Purge messages but keep stream
nats stream purge dex-swaps
```

### Consumer Management

#### Create Consumers

Create a durable push consumer for the ClickHouse sink:
```bash
nats consumer add dex-swaps clickhouse-sink \
  --filter="dex.sol.swaps" \
  --ack=explicit \
  --wait=30s \
  --max-deliver=3 \
  --deliver=all \
  --replay=instant
```

Create a pull consumer for API queries:
```bash
nats consumer add dex-swaps api-query \
  --pull \
  --filter="dex.sol.swaps" \
  --ack=explicit \
  --max-deliver=5 \
  --replay=instant
```

#### List Consumers

```bash
# List consumers for a stream
nats consumer ls dex-swaps

# View consumer details
nats consumer info dex-swaps clickhouse-sink

# View consumer state
nats consumer state dex-swaps clickhouse-sink
```

#### Delete Consumers

```bash
nats consumer rm dex-swaps clickhouse-sink
```

### Publishing Messages

Publish a test swap event:
```bash
# With deduplication ID
nats pub dex.sol.swaps \
  --header="Msg-Id:501:12345678:abc123:0" \
  '{"slot":12345678,"signature":"abc123","pool":"raydium_xyz","amount_in":1000}'

# Publish from file
nats pub dex.sol.swaps < test-swap.json
```

### Consuming Messages

Subscribe to a subject (ephemeral consumer):
```bash
nats sub dex.sol.swaps
```

Pull messages from a durable consumer:
```bash
# Pull 10 messages
nats consumer next dex-swaps clickhouse-sink --count=10

# Pull with acknowledgment
nats consumer next dex-swaps clickhouse-sink --ack
```

### Monitoring & Diagnostics

#### Stream Metrics

```bash
# Real-time stream stats
nats stream report

# Consumer report across all streams
nats consumer report

# Server info
nats server info

# Check server health
nats server ping
```

#### Message Inspection

```bash
# Get a specific message by sequence
nats stream get dex-swaps 12345

# View first message
nats stream get dex-swaps --first

# View last message
nats stream get dex-swaps --last
```

#### Benchmarking

```bash
# Benchmark publishing
nats bench dex.sol.swaps --pub 5 --size 1024 --msgs 100000

# Benchmark subscribing
nats bench dex.sol.swaps --sub 5 --msgs 100000
```

### Backup & Restore

Create a backup snapshot:
```bash
# Backup stream to directory
nats stream backup dex-swaps /backup/dex-swaps-$(date +%Y%m%d)
```

Restore from backup:
```bash
nats stream restore dex-swaps /backup/dex-swaps-20250116
```

## Message Deduplication

The indexer uses message IDs in the format:
```
Msg-Id: 501:<slot>:<signature>:<tx_index>
```

Example:
```
Msg-Id: 501:234567890:5Xk2Fj...9aB:0
```

This ensures exactly-once processing even with retries. The `501` prefix represents the Solana cluster ID (mainnet-beta).

## Configuration Files

Stream and consumer configurations can be stored as JSON for reproducibility:

**`stream-dex-swaps.json`**:
```json
{
  "name": "dex-swaps",
  "subjects": ["dex.sol.swaps"],
  "retention": "workqueue",
  "storage": "file",
  "replicas": 3,
  "max_age": 604800000000000,
  "max_msg_size": 1048576,
  "duplicate_window": 3600000000000,
  "discard": "old"
}
```

Apply from JSON:
```bash
nats stream add --config=stream-dex-swaps.json
```

## Troubleshooting

### Stream has no consumers
```bash
nats consumer add dex-swaps <consumer-name>
```

### Messages piling up (consumer lag)
```bash
# Check consumer state
nats consumer info dex-swaps clickhouse-sink

# Check pending messages
nats stream state dex-swaps

# Scale consumers or increase ack wait time
nats consumer edit dex-swaps clickhouse-sink --wait=60s
```

### Duplicate messages detected
Verify `Msg-Id` header is set correctly:
```bash
nats stream get dex-swaps --last
```

### Connection refused
```bash
# Check NATS server is running
nats server ping

# Verify server URL
nats context info
```

## Production Checklist

Before deploying to production:

- [ ] Enable TLS for client connections
- [ ] Configure authentication (JWT, credentials, or NKeys)
- [ ] Set up monitoring (Prometheus exporter on `:7777/metrics`)
- [ ] Configure stream retention policies based on disk capacity
- [ ] Set up automated backups
- [ ] Enable replicas (3 or 5 for high availability)
- [ ] Configure resource limits (max connections, memory)
- [ ] Set up alerting for consumer lag and stream capacity

## References

- [NATS CLI Documentation](https://docs.nats.io/using-nats/nats-tools/nats_cli)
- [JetStream Guide](https://docs.nats.io/nats-concepts/jetstream)
- [Exactly-Once Semantics](https://docs.nats.io/using-nats/developer/develop_jetstream/model_deep_dive#message-deduplication)
