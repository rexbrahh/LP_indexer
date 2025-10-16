# Meteora Decoder Scaffold

This package hosts the Meteora swap decoder outlined in the locked Solana
liquidity indexer specification. Meteora supports both Dynamic Liquidity
Pools (DLMM) and Constant Product Market Maker (CPMM) pools, and each swap must
be normalised into the canonical `dex.sol.v1.SwapEvent` protobuf so the rest of
the pipeline remains agnostic to the source venue.

## Responsibilities

* Parse Meteora instructions/logs and determine whether the event originated
  from a DLMM or CPMM pool.
* Normalise token orientation (base/quote) following the canonical pair rules
  shared across the repo.
* Surface optional pool state information such as DLMM virtual reserves or CPMM
  reserves when emitted.
* Emit the canonical protobuf structures so the Go sinks and C++ candle engine
  can consume identical payloads regardless of the upstream decoder.

## Current Status

Only scaffolding exists today:

* `types.go` defines the primary data structures (`SwapEvent`, `PoolKind`,
  etc.) plus constants for the Meteora program IDs recorded in the spec.
* `decoder.go` exposes stubs for log/instruction decoding and returns
  `ErrNotImplemented` until the actual parsing logic lands.
* `proto.go` converts a `SwapEvent` into the canonical protobuf message, ready
  for use once decoding is implemented.

Upcoming work will wire in the binary decoding routines, pair normalisation, and
unit tests with fixture data captured from Meteora pools.
