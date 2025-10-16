#pragma once

#include "candle_types.hpp"
#include <mutex>
#include <string>
#include <vector>

namespace candle {

/// Abstract publisher interface for emitting finalized/provisional candles.
class CandlePublisher {
public:
  virtual ~CandlePublisher() = default;

  /// Publish a candle for the given pair and timeframe.
  virtual void publish(const std::string &pair_id, WindowSize window,
                       const Candle &candle) = 0;
};

/// In-memory publisher used for tests and bootstrap scaffolding.
class InMemoryPublisher : public CandlePublisher {
public:
  void publish(const std::string &pair_id, WindowSize window,
               const Candle &candle) override;

  std::vector<Candle> snapshot() const;

private:
  mutable std::mutex mutex_;
  std::vector<Candle> emitted_;
};

} // namespace candle
