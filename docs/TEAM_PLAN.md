# Parallel Work Plan (Current Sprint)

## Engineer A – Stream & Metadata Bootstrap
- Harden Yellowstone client (TLS, token auth) and demo command.
- Slot cache API documentation (`ingestor/common`).
- Coordinate with decoder owners on canonical IDs.

## Engineer B – Raydium Decoder
- Maintain fixtures under `decoder/raydium/testdata/`.
- Ensure amount/price math stays in sync with canonical pair resolver.
- Add benchmarks as new instruction variants ship.

## Engineer C – Orca Whirlpools Decoder
- Share mint metadata helper with Raydium; avoid duplicated registries.
- Validate price math vs recorded transactions; keep tests deterministic.

## Engineer D – Candle Engine
- Extend timing wheel to cover 1s window; emit to NATS publisher stub.
- Grow test coverage (watermark behavior, multi-window finalization).

## Engineer E – Ops & Sinks
- `make ops.jetstream.verify` in CI; keep `scripts/jetstream-validate.sh` up-to-date.
- Expand ClickHouse config coverage (retry knobs, metrics).

## Engineer F – API Skeleton
- Redis cache integration, OpenAPI examples, readiness/health endpoints.
- Add rate limiting & auth middleware stubs.

### Cadence
- **Kickoff (30 min):** align on schema and playback ranges.
- **Midpoint sync (15 min):** unblock cross-module dependencies.
- **EOD checkpoint (15 min):** commit summaries + open issues.

### Expectations
- Branch per feature (`feat/<area>-<slug>`), PR ≤400 LOC when possible.
- Run `make fmt lint build` before pushing; include doc updates in same PR.
- Document any interface change in `docs/` and notify #market-data-indexer.
