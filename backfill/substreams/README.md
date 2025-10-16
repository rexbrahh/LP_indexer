# Substreams Backfill Scaffold

This directory holds the placeholder assets for StreamingFast Substreams that will
power historical backfills. The goal is to decode Raydium, Orca Whirlpools, and
Meteora swaps/pool snapshots into the canonical `dex.sol.v1` protobuf messages
so that historical and live flows use identical math.

## Contents

- `substreams.yaml` – manifest skeleton declaring `map_swaps`, `store_trades`,
  and `map_pool_snapshots` modules.
- `proto/` – (future) protobuf module descriptors for Substreams output.
- `modules/` – (future) Rust-based modules that transform Solana blocks into
  `SwapEvent` and `PoolSnapshot` streams.

## Next Steps

1. Implement `map_swaps` to decode Raydium/Orca/Meteora instructions into
   `dex.sol.v1.SwapEvent`.
2. Implement `store_trades` to accumulate swaps for downstream sinks.
3. Implement `map_pool_snapshots` for pool-level state.
4. Add CI job to build the Substreams package and publish artifacts.
