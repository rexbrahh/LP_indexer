# Backfill Strategy

## Workflow
1. **Substreams** – Run `substreams.yaml` modules (`map_swaps`, `store_trades`, `map_pool_snapshots`) for desired slot range.
2. **Sink** – Pipe output to ClickHouse (`substreams-sink-clickhouse`) or Parquet writer for cold storage.
3. **Orchestrator** – Track range checkpoints, parallelize ranges while respecting provider and sink quotas.

## Acceptance
- Replayed day matches live pipeline within 1% volume and 0.1% trade count.
- Candles recomputed from backfill match live engine math (q32.32 accuracy).
- Checkpoints stored (Postgres/ClickHouse) to resume from last completed slot.

## Commands
```bash
substreams run -e mainnet.sol.streamingfast.io:443 backfill/substreams/substreams.yaml map_swaps \
  --start-block <slot_start> --stop-block <slot_end> \
  | substreams-sink-clickhouse --dsn "$CLICKHOUSE_DSN" --table trades --flush-interval 2s
```

## Considerations
- Large slot ranges generate heavy IO; throttle to avoid JetStream catch-up delays.
- Always run parity checks vs legacy dataset before enabling new consumers.
