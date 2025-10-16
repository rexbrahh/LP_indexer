#include "publisher.hpp"
#include "candle_worker.hpp"

namespace candle {

void InMemoryPublisher::publish(const std::string &pair_id, WindowSize window,
                                const Candle &candle) {
  (void)pair_id;
  (void)window;
  std::lock_guard<std::mutex> lock(mutex_);
  emitted_.push_back(candle);
}

std::vector<Candle> InMemoryPublisher::snapshot() const {
  std::lock_guard<std::mutex> lock(mutex_);
  return emitted_;
}

} // namespace candle
