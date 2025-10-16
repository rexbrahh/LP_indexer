# Candle Engine C++ Module

High-performance C++20 candle aggregation engine for real-time OHLCV computation across multiple time windows.

## Overview

The candle engine ingests normalized DEX swap events and maintains stateful candle (OHLCV) data for each trading pair across multiple time granularities (1m, 5m, 15m, 1h, 4h, 1d). It uses fixed-point arithmetic (Q32.32) to ensure deterministic, overflow-safe price calculations.

## Architecture

### Components

1. **CandleWorker** (`candle_worker.hpp`, `candle_worker.cpp`)
   - Main entry point for processing trade events
   - Manages sharding of trading pairs across multiple shards for lock contention reduction
   - Routes incoming trades to appropriate shard based on consistent hashing
   - Emits candles via the pluggable publisher interface (defaults to in-memory sink; NATS integration pending)
2. **Publisher** (`publisher.hpp`, `publisher.cpp`)
   - Abstract interface for downstream sinks (NATS, ClickHouse, Parquet)
   - Default implementation stores emitted candles in-memory for tests and local inspection
   - Future implementations will serialize to `dex.sol.v1.Candle` protobufs and forward to JetStream

3. **Shard** (defined in `candle_worker.hpp`)
   - Owns a subset of trading pairs' candle windows
   - Each shard maintains independent locks to enable parallel processing
   - Maps `pair_id → vector<CandleWindow>` (one window per time granularity)

4. **CandleWindow** (defined in `candle_worker.hpp`)
   - Manages candles for a specific pair and time window size
   - Stores `map<window_start_time, Candle>` for historical windows
   - Thread-safe updates with per-window mutex

5. **FixedPoint** (`fixed_point.hpp`, `fixed_point.cpp`)
   - Q32.32 fixed-point arithmetic utilities
   - 128-bit intermediate calculations for multiplication/division to prevent overflow
   - Supports deterministic price/volume operations without floating-point errors

### Data Structures

#### Shard Structure
```cpp
struct Shard {
    uint32_t shard_id;
    // Key: pair_id (e.g., "SOL/USDC")
    // Value: vector of 6 CandleWindow objects (one per time granularity)
    std::map<std::string, std::vector<std::shared_ptr<CandleWindow>>> windows;
    std::mutex mutex;  // Protects windows map
};
```

**Sharding Strategy:**
- Pairs are distributed via FNV-1a hash: `shard_index = hash(pair_id) % num_shards`
- Default: 16 shards (configurable via constructor)
- Ensures consistent shard assignment for same pair across restarts

#### CandleWindow Structure
```cpp
struct CandleWindow {
    WindowSize window_size;  // 60s, 300s, 900s, 3600s, 14400s, 86400s
    std::string pair_id;
    std::map<uint64_t, Candle> candles;  // Key: window_start_time (Unix seconds)
    std::mutex mutex;  // Protects candles map
};
```

**Window Computation:**
- `window_start = (timestamp / window_size) * window_size`
- Example: timestamp=1700000065, window_size=60 → window_start=1700000040

#### Candle Structure
```cpp
struct Candle {
    uint64_t open_time;       // Window start timestamp
    uint64_t close_time;      // Window end timestamp
    FixedPrice open;          // First trade price in window (Q32.32)
    FixedPrice high;          // Highest trade price (Q32.32)
    FixedPrice low;           // Lowest trade price (Q32.32)
    FixedPrice close;         // Last trade price (Q32.32)
    FixedPrice volume;        // Total base token volume (Q32.32)
    FixedPrice quote_volume;  // Total quote token volume (Q32.32)
    uint32_t trades;          // Number of trades in window
    bool provisional;         // True if window hasn't been finalized yet
};
```

## Candle Finalization & Watermark Behavior

### Provisional vs Finalized Candles

All candles start as **provisional** (`provisional = true`) when first created. They transition to **finalized** (`provisional = false`) when:

1. The candle's `close_time` has passed (window is closed)
2. A background timing wheel detects the window closure
3. The candle is emitted to the sink and removed from memory

### Watermark Tracking

Each `CandleWindow` maintains a `last_trade_time` watermark:

- **Updated on every trade:** `last_trade_time = max(last_trade_time, current_timestamp)`
- **Used for finalization:** Candles whose `close_time <= current_watermark` are eligible for finalization
- **Background thread:** A timing wheel runs every 1 second to check for candles ready to finalize

**Finalization Process:**
1. Timing wheel wakes up every 1 second
2. For each shard → pair → window:
   - Get all candles where `close_time <= watermark`
   - Flip `provisional` flag to `false`
   - Emit candle via `emit_candle()`
   - Remove from `candles` map to free memory

**Benefits:**
- Ensures 1m windows finalize within 1 second of closure
- Prevents unbounded memory growth from old candles
- Emitted candles have deterministic finalization semantics

## Threading & Concurrency Model

### Current Implementation (Phase 1)

**Synchronous Processing:**
- `CandleWorker::on_trade()` processes trades **synchronously** in the caller's thread
- Each trade update:
  1. Hashes `pair_id` to determine shard index
  2. Acquires shard mutex
  3. Gets or creates 6 CandleWindow objects for the pair
  4. Releases shard mutex
  5. For each window: acquires window mutex, updates candle, releases mutex
  6. Updates `last_trade_time` watermark

**Timing Wheel (Finalization Thread):**
- Background thread runs `finalize_loop()` every 1 second
- Iterates through all shards/pairs/windows
- Calls `finalize_old_candles(watermark)` to finalize closed windows
- Emits finalized candles to in-memory sink (provisional: will be NATS later)

**Lock Granularity:**
- **Shard-level mutex:** Protects the `windows` map during pair lookup/creation
- **Window-level mutex:** Protects individual `candles` map during updates and finalization
- **Emitted candles mutex:** Protects the in-memory sink for emitted candles
- Minimizes contention: different pairs in same shard can update windows in parallel after initial lookup

### Planned Implementation (Phase 2)

**Asynchronous Thread Pool:**
```
[Upstream Decoder]
       ↓ (produces trade events)
[Lock-Free SPSC Queue per Shard] (bounded ring buffers)
       ↓
[Worker Thread Pool] (1 thread per shard)
       ↓ (processes trades from queue)
[CandleWindow Updates]
       ↓ (on window close)
[emit_candle() → NATS JetStream]
```

**Components:**
1. **Per-Shard Lock-Free Queue**
   - Single-producer (upstream decoder), single-consumer (shard worker thread)
   - Bounded queue (e.g., 16384 slots) with backpressure to decoder
   - Trade events serialized as: `{pair_id, timestamp, price, base_amt, quote_amt}`

2. **Worker Thread Pool**
   - One dedicated thread per shard (default: 16 threads for 16 shards)
   - Each thread:
     - Polls its shard's queue (busy-wait or futex-based signaling)
     - Processes trades in FIFO order
     - Detects window closes (current trade timestamp >= next window boundary)
     - Calls `emit_candle()` for closed windows

3. **Candle Emission**
   - Worker thread packages closed candle into protobuf
   - Publishes to NATS JetStream subject: `dex.sol.candles.{pair_id}.{window_size}`
   - Sets `Msg-Id: "501:{slot}:{sig}:{index}"` for exactly-once delivery

**Benefits:**
- Decouples upstream decoder (Go) from C++ processing latency
- Eliminates cross-shard lock contention
- Scales to ~100k trades/sec with single-digit millisecond latency

## Fixed-Point Arithmetic (Q32.32)

### Representation
- 64-bit signed integer: 32 bits integer part + 32 bits fractional part
- `1.0 = 0x0000000100000000 = 4294967296`
- `0.5 = 0x0000000080000000 = 2147483648`
- Range: ~±2 billion with ~0.23 nanoscale precision

### Operations
- **Addition/Subtraction:** Direct int64 arithmetic (no special handling)
- **Multiplication:** Uses 128-bit intermediate to avoid overflow
  - `(a * b) >> 32` computed with 128-bit product
- **Division:** Uses 128-bit intermediate for precision
  - `(a << 32) / b` computed with 128-bit dividend

### Safety Guarantees
- All multiply/divide operations detect overflow and throw `std::overflow_error`
- Division by zero throws `std::domain_error`
- Helper functions in `detail` namespace handle 128-bit arithmetic emulation

## Building & Testing

### Dependencies
- C++20 compiler (Clang 17+ or GCC 12+)
- GoogleTest (for unit tests)

### Build Commands
```bash
# From repository root
make candle-cpp        # Build library
make candle-cpp-test   # Build and run tests
```

### Running Tests
```bash
cd state/candle_cpp/tests
./fixed_point_test
```

**Test Coverage:**
- Q32.32 construction from int/double
- Addition, subtraction, comparison operators
- Multiplication with 128-bit overflow safety
- Division with zero-check and 128-bit precision
- Edge cases: large values, negative numbers, fractional precision

## Integration Points

### Upstream (Go Decoder)
```go
// Example: calling C++ from Go via cgo
import "C"

func onSwap(pairID string, slot uint64, price, baseAmt, quoteAmt int64) {
    C.candle_worker_on_trade(
        C.CString(pairID),
        C.uint64_t(slot),
        C.int64_t(price),    // Already in Q32.32 from price_cpp
        C.int64_t(baseAmt),
        C.int64_t(quoteAmt),
    )
}
```

### Downstream (NATS Emission - TBD)
- `emit_candle()` currently logs to stdout (provisional stub)
- Future: serialize to protobuf `dex.sol.v1.Candle` and publish to NATS
- Subject: `dex.sol.candles.{pair_id}.{window_seconds}`
- Msg-Id: `"501:{slot}:{sig}:{index}"` for deduplication

## Performance Targets

- **Throughput:** 100k trades/sec sustained (per worker thread)
- **Latency:** <5ms p99 from trade ingestion to candle update
- **Memory:** ~1 MB per pair (6 windows × ~30 historical candles each)
- **Scalability:** Linear with shard count (16 shards = 16x parallel capacity)

## Future Work

1. **Thread Pool Implementation**
   - Add lock-free SPSC queues (e.g., Boost.Lockfree or folly::ProducerConsumerQueue)
   - Spawn worker threads in `CandleWorker::start()`
   - Graceful shutdown in `CandleWorker::stop()`

2. **NATS Integration**
   - Link against `nats.c` library
   - Implement protobuf serialization in `emit_candle()`
   - Add retry logic for transient NATS failures

3. **Candle Pruning**
   - Add TTL for historical candles (e.g., keep last 1000 windows per pair)
   - Background thread to periodically evict old candles

4. **Observability**
   - Expose metrics: trades/sec, candle emissions, queue depths
   - Add structured logging (e.g., spdlog integration)

5. **Backfill Support**
   - Accept historical trades out-of-order (requires window locking refinement)
   - Emit backfill candles to separate NATS subject for differentiation
