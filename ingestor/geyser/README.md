# Geyser Ingestor

This package implements a robust Yellowstone Geyser gRPC client for streaming Solana blockchain data with automatic reconnection and replay capabilities.

## Overview

The Geyser ingestor subscribes to on-chain events from a Yellowstone Geyser endpoint, filtering for specific Solana program IDs (Raydium, Orca, Meteora). It provides resilient streaming with automatic reconnection and historical replay to ensure no data loss during network interruptions.

## Features

- **Automatic Reconnection**: Seamlessly reconnects to the Geyser stream on connection failures
- **64-Slot Replay Window**: On reconnect, replays the last 64 slots to ensure continuity
- **Program Filtering**: Subscribes only to accounts owned by configured DEX programs
- **Slot Metadata Tracking**: Subscribes to slot and block metadata for timing information
- **Configurable via Environment**: Easy configuration through environment variables and YAML

## Configuration

### Environment Variables

```bash
# Required
GEYSER_ENDPOINT="grpc.chainstack.com:443"  # Geyser gRPC endpoint
GEYSER_API_KEY="your-api-key-here"         # Authentication key

# Optional
PROGRAMS_YAML_PATH="ops/programs.yaml"      # Path to programs filter config
```

### Programs Configuration

Program filters are defined in `ops/programs.yaml`:

```yaml
programs:
  raydium_amm: 675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8
  orca_whirlpool: whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc
  meteora_pools: LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo
```

## Usage

```go
import (
    "log"
    "github.com/yourusername/lp-indexer/ingestor/geyser"
)

func main() {
    // Load configuration
    cfg, err := geyser.LoadConfig("ops/programs.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create client
    client, err := geyser.NewClient(cfg)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Connect to Geyser
    if err := client.Connect(); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }

    // Subscribe starting from slot 0 (or latest known slot)
    updateCh, errCh := client.Subscribe(0)

    // Process updates
    for {
        select {
        case update := <-updateCh:
            // Handle update (account, transaction, slot, block, etc.)
            processUpdate(update)
        case err := <-errCh:
            // Handle error (non-fatal, connection will retry)
            log.Printf("Stream error: %v", err)
        }
    }
}
```

## Reconnection & Replay Behavior

### Reconnection Strategy

When a connection is lost or an error occurs:

1. The client logs the error and current slot position
2. Waits for **5 seconds** (configurable via `ReconnectBackoff`)
3. Attempts to reconnect and resubscribe
4. Repeats indefinitely until shutdown

### 64-Slot Replay Window

On reconnection, the client rewinds **64 slots** from the last processed slot to ensure no events are missed:

```
Last processed slot: 1000
Reconnect replay starts at: 936 (1000 - 64)
```

This window accounts for:
- Network latency during disconnection
- Potential reorgs or uncle blocks
- Buffer for deduplication logic downstream

**Important**: Downstream consumers must implement deduplication using the slot + signature to handle replayed events.

## Subscription Filters

The client subscribes to:

- **Accounts**: Filters by program owner (Raydium, Orca, Meteora)
- **Slots**: All slots for timing metadata
- **Block Metadata**: Block timestamps and parent slot info

It does NOT subscribe to:
- Raw transactions (only account updates)
- Entry data
- Full blocks (only metadata)

This reduces bandwidth while capturing all relevant DEX state changes.

## Error Handling

### Non-Fatal Errors

These errors trigger reconnection but do not stop the client:
- Network timeouts
- gRPC connection errors
- Stream EOF from server

Errors are sent to the `errCh` channel for logging/monitoring.

### Fatal Errors

These errors cause client shutdown:
- Context cancellation (via `Close()`)
- Invalid configuration

## Integration with Slot Cache

The Geyser client works with the `ingestor/common.SlotTimeCache` to maintain slot → timestamp mappings:

```go
import "github.com/yourusername/lp-indexer/ingestor/common"

cache := common.NewMemorySlotTimeCache()

for update := range updateCh {
    switch u := update.UpdateOneof.(type) {
    case *pb.SubscribeUpdate_Slot:
        // Store slot timing
        cache.Set(u.Slot.Slot, time.Unix(u.Slot.Timestamp, 0))
    }
}
```

## Testing

Run unit tests:

```bash
go test ./ingestor/geyser/...
```

Run the Geyser demo:

```bash
export GEYSER_ENDPOINT="solana-mainnet.core.chainstack.com:443"
export GEYSER_API_KEY="your-chainstack-api-key"
go run ./cmd/ingestor/geyser-demo/main.go
```

**Required Environment Variables:**
- `GEYSER_ENDPOINT`: The Yellowstone gRPC endpoint (e.g., `solana-mainnet.core.chainstack.com:443`)
- `GEYSER_API_KEY`: Your Chainstack API key for authentication

The demo will connect to the Geyser endpoint, subscribe to slot updates and account changes for configured DEX programs, and log slot numbers to stdout.

## Performance Considerations

- **Message Size**: Configured for 128MB max message size to handle large account updates
- **Channel Buffering**: 100-message buffer on update channel to prevent blocking
- **Goroutine Safety**: All operations are thread-safe via mutex locks

## Monitoring

Key metrics to track:

- Reconnection frequency
- Replay window size (should stay near 64 slots)
- Update processing latency
- Error rate from `errCh`

## Known Limitations

- Does not persist slot position to disk (in-memory only)
- Replay window is fixed at 64 slots (not configurable per connection)

## Future Enhancements

- [ ] Persistent slot checkpoint (Redis/PostgreSQL)
- [ ] Dynamic replay window based on network conditions
- [ ] Prometheus metrics exporter
- [ ] TLS certificate management
- [ ] Rate limiting and backpressure handling
