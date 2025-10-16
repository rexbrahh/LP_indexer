# Candle Engine Blueprint

## Responsibilities
- Ingest normalized `SwapEvent` and `BlockHead` messages via NATS JetStream.
- Maintain pool-level and pair-level rolling windows for 1s/1m/5m/1h/1d.
- Publish provisional candles immediately; finalize once watermarks pass (`confirmed` → provisional, `finalized` → provisional=false).

## Architecture
- Sharded by `pool_id` (default 16 shards) to avoid lock contention.
- Each `CandleWindow` stores `std::map<window_start, Candle>`; update path uses per-window mutex.
- Timing wheel background thread wakes every second, finalizes windows whose `close_time <= watermark`.
- Provisional candles emitted to in-memory sink for now; future step publishes protobuf payloads to `dex.sol.candle.*` via NATS.

## Data Model
```
struct Candle {
  uint64_t open_time, close_time;
  FixedPrice open, high, low, close; // Q32.32
  FixedPrice volume, quote_volume;
  uint32_t trades;
  bool provisional;
}
```
- `FixedPoint` (Q32.32) uses 128-bit intermediates for multiply/divide.
- `CandleWindow::last_trade_time` tracks watermark eligibility.

## Finalization Flow
1. Background thread obtains watermark (current wall clock or block time).
2. For each window, call `finalize_old_candles(watermark)`.
3. Flip `provisional=false`, emit, erase from active map.
4. Work item left for NATS publisher integration.

## Tests
- `candle_finalize_test.cpp` validates watermark update, provisional flip, multi-window finalization, and in-memory emission.
- Add regression tests whenever price/volume arithmetic changes.

## TODOs
- Publish to JetStream (`dex.sol.candle.pool.<tf>`, `dex.sol.candle.pair.<tf>`).
- Persist emitted candles to ClickHouse and Parquet sinks.
- Surface Prometheus metrics (latency, provisional count, finalize lag).
