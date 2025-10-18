#include "candle_worker.hpp"
#include "publisher.hpp"
#include <gtest/gtest.h>

namespace candle {

class StubPublisher : public CandlePublisher {
public:
  void publish(const std::string &pair_id, WindowSize window,
               const Candle &candle) override {
    std::lock_guard<std::mutex> lock(mutex_);
    last_pair_id_ = pair_id;
    last_window_ = window;
    last_candle_ = candle;
    call_count_++;
  }

  std::string last_pair() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return last_pair_id_;
  }

  WindowSize last_window() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return last_window_;
  }

  Candle last_candle() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return last_candle_;
  }

  int calls() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return call_count_;
  }

private:
  mutable std::mutex mutex_;
  std::string last_pair_;
  WindowSize last_window_{WindowSize::SEC_1};
  Candle last_candle_;
  int call_count_{0};
};

TEST(CandlePublisherDispatchTest, EmitsViaCustomPublisher) {
  CandleWorker worker(2);
  auto stub = std::make_shared<StubPublisher>();
  worker.set_publisher(stub);

  Candle candle;
  candle.open_time = 1700000000;
  candle.close_time = 1700000060;
  candle.open = 100;
  candle.high = 110;
  candle.low = 90;
  candle.close = 105;
  candle.volume = 250;
  candle.quote_volume = 500;
  candle.trades = 3;
  candle.provisional = false;

  worker.emit_candle("SOL_USDC", WindowSize::MIN_1, candle);

  EXPECT_EQ(stub->calls(), 1);
  EXPECT_EQ(stub->last_pair(), "SOL_USDC");
  EXPECT_EQ(stub->last_window(), WindowSize::MIN_1);
  auto recorded = stub->last_candle();
  EXPECT_EQ(recorded.open, candle.open);
  EXPECT_EQ(recorded.close, candle.close);
  EXPECT_EQ(recorded.volume, candle.volume);
  EXPECT_EQ(recorded.trades, candle.trades);
}

} // namespace candle
