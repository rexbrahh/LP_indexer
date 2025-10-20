# Geyser Demo

This demo shows how to connect to a Yellowstone Geyser endpoint, stream slot and account updates, and observe the slot→timestamp cache in action. It provides a minimal harness to quickly validate connectivity and understand the ingest pipeline behaviour before wiring the full decoder and sink stacks.

## Configuration

Set the following environment variables (Chainstack example endpoints shown):

```
GEYSER_ENDPOINT=solana-mainnet.core.chainstack.com:443
GEYSER_API_KEY=your_chainstack_api_key
GEYSER_PROGRAMS_JSON=ops/geyser_programs_demo.json (optional)
```

- If `GEYSER_PROGRAMS_JSON` is omitted, the demo subscribes to a small default program set (Raydium, Orca, Meteora).
- Credentials are pulled from env; do not hardcode keys in source control.

## Running the Demo

```bash
GEYSER_ENDPOINT=... GEYSER_API_KEY=... make demo.geyser
```

or directly:

```bash
GEYSER_ENDPOINT=... GEYSER_API_KEY=... go run ./cmd/ingestor/geyser-demo
```

The demo performs:
1. Load configuration and program filters.
2. Establish a secure connection (TLS) and authenticate via API key.
3. Subscribe to slot, account, and block metadata updates.
4. Log each slot with parent slots, maintaining the slot→timestamp cache.
5. Respond to Geyser ping messages to keep the stream alive.
6. Handle reconnects on errors and graceful shutdown on SIGINT/SIGTERM.

Example output:

```
2025-10-21T00:30:12Z Connected to Geyser endpoint solana-mainnet.core.chainstack.com:443
2025-10-21T00:30:13Z Slot 175839256 (parent: 175839255) - total slots: 1
2025-10-21T00:30:14Z Account update at slot 175839256: Hx3K...9F7d (owner: 675kPX9M...Mp8)
2025-10-21T00:30:15Z Block meta at slot 175839256: block time 1729480215
```

## Slot Cache

The demo uses the shared slot cache from `ingestor/geyser/service.go`. It tracks:
- Latest slot per fork.
- Parent relationships for rollback detection.
- Timestamp mapping (`slot → time.Time`), used downstream to normalize event timestamps.

On reorg detection, prior slots are marked provisional/undo as required by the downstream pipeline.

## Program Filters

Program filters default to a small curated set, but can be overridden via JSON:

```json
{
  "raydium_amm": "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8",
  "orca_whirlpool": "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc",
  "meteora_pools": "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo"
}
```

The demo merges these with the default set. Production configurations should come from `ops/programs.yaml`.

## Next Steps

- Integrate the decoded swap events into JetStream once the decoder pipeline is ready.
- Expand logging to include canonical `SwapEvent` payloads for debugging.
- Add metrics (slot lag, reconnect count) consistent with docs/OBSERVABILITY.md.
