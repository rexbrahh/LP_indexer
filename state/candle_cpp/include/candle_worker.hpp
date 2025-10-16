#pragma once

#include <atomic>
#include <cstdint>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace candle {

// Forward declarations
struct Shard;
struct CandleWindow;
class CandleWorker;

/// Fixed-point price representation (Q32.32)
using FixedPrice = int64_t;

/// Time granularity for candle windows (in seconds)
enum class WindowSize : uint32_t {
  SEC_1 = 1,
  MIN_1 = 60,
  MIN_5 = 300,
  MIN_15 = 900,
  HOUR_1 = 3600,
  HOUR_4 = 14400,
  DAY_1 = 86400
};

/// Represents a single candle for a time window
struct Candle {
  uint64_t open_time;      // Unix timestamp (seconds)
  uint64_t close_time;     // Unix timestamp (seconds)
  FixedPrice open;         // First trade price in window
  FixedPrice high;         // Highest trade price in window
  FixedPrice low;          // Lowest trade price in window
  FixedPrice close;        // Last trade price in window
  FixedPrice volume;       // Total volume (base token)
  FixedPrice quote_volume; // Total volume (quote token)
  uint32_t trades;         // Number of trades
  bool provisional;        // True if window hasn't been finalized yet

  Candle()
      : open_time(0), close_time(0), open(0), high(0), low(0), close(0),
        volume(0), quote_volume(0), trades(0), provisional(true) {}
};

/// Time-windowed candle aggregator for a specific pair/window combination
struct CandleWindow {
  WindowSize window_size;
  std::string pair_id;
  std::map<uint64_t, Candle> candles; // key: window_start_time
  std::mutex mutex;
  uint64_t last_trade_time; // Watermark for finalization

  CandleWindow(WindowSize ws, const std::string &pid)
      : window_size(ws), pair_id(pid), last_trade_time(0) {}

  /// Update or create candle for the given trade
  void update(uint64_t timestamp, FixedPrice price, FixedPrice base_amount,
              FixedPrice quote_amount);

  /// Get window start time for a given timestamp
  uint64_t get_window_start(uint64_t timestamp) const;

  /// Finalize candles older than watermark and return them
  std::vector<Candle> finalize_old_candles(uint64_t watermark);
};

/// Shard: owns a subset of pair_id's candle windows
struct Shard {
  uint32_t shard_id;
  // Map: pair_id -> vector of CandleWindow (one per WindowSize)
  std::map<std::string, std::vector<std::shared_ptr<CandleWindow>>> windows;
  std::mutex mutex;

  explicit Shard(uint32_t id) : shard_id(id) {}

  /// Get or create candle windows for a pair
  std::vector<std::shared_ptr<CandleWindow>> &
  get_or_create_windows(const std::string &pair_id);

  /// Process a trade update
  void process_trade(const std::string &pair_id, uint64_t timestamp,
                     FixedPrice price, FixedPrice base_amount,
                     FixedPrice quote_amount);
};

/// Main worker that manages shards and processes incoming trades
class CandleWorker {
public:
  /// Constructor
  /// @param num_shards Number of shards to partition pairs across
  explicit CandleWorker(uint32_t num_shards = 16);

  /// Destructor
  ~CandleWorker();

  /// Start the worker threads
  void start();

  /// Stop the worker threads
  void stop();

  /// Process a trade event (thread-safe)
  /// @param pair_id Normalized pair identifier (e.g., "SOL/USDC")
  /// @param timestamp Unix timestamp in seconds
  /// @param price Trade price (Q32.32 fixed-point)
  /// @param base_amount Base token amount (Q32.32 fixed-point)
  /// @param quote_amount Quote token amount (Q32.32 fixed-point)
  void on_trade(const std::string &pair_id, uint64_t timestamp,
                FixedPrice price, FixedPrice base_amount,
                FixedPrice quote_amount);

  /// Emit completed candles to in-memory sink (provisional: will be NATS later)
  /// Will be called when a candle window closes
  void emit_candle(const std::string &pair_id, WindowSize window_size,
                   const Candle &candle);

  /// Get shard for a given pair_id (consistent hashing)
  uint32_t get_shard_for_pair(const std::string &pair_id) const;

  /// Get emitted candles (for testing)
  std::vector<Candle> get_emitted_candles() const;

private:
  uint32_t num_shards_;
  std::vector<std::unique_ptr<Shard>> shards_;
  std::atomic<bool> running_;

  // Thread pool for processing (future work)
  std::vector<std::thread> worker_threads_;

  // Timing wheel for finalization
  std::thread finalize_thread_;

  // In-memory sink for emitted candles (provisional)
  std::vector<Candle> emitted_candles_;
  mutable std::mutex emitted_mutex_;

  /// Initialize shards
  void init_shards();

  /// Hash function for pair_id -> shard mapping
  uint32_t hash_pair_id(const std::string &pair_id) const;

  /// Background finalization loop (timing wheel)
  void finalize_loop();
};

} // namespace candle
