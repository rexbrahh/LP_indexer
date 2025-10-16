#pragma once

#include <cstdint>

namespace candle {

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

} // namespace candle
