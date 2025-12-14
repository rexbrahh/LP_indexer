# Testing Strategy

## Unit Tests
- Go packages: `go test ./...`
  - Slot cache concurrency (`ingestor/common/slot_cache_test.go`)
  - Raydium & Orca decoders fixture tests
  - ClickHouse writer batch/validation tests
- C++: GoogleTest under `state/candle_cpp/tests/`
  - Fixed-point arithmetic
  - Candle finalization behaviour

## Integration Tests (future work)
- Stream replay harness feeding recorded swaps through decoder + candle pipeline.
- ClickHouse + JetStream integration via docker-compose.

## Performance Tests
- Candle engine benchmark to sustain 100k events/s (to be scripted).
- Ingestor latency metrics (provisional vs finalized).

## Backfill Validation
- Replay known slot ranges; compare results against legacy data and Substreams output.
- Check `docs/REPLAY_TEST.md` for harness expectations.

## CI Checklist
- `go test ./...`
- `make lint`
- `scripts/build_candles.sh && ./build/candle_cpp/candle_service` tests

Add new tests alongside features; update docs when behaviour changes.
