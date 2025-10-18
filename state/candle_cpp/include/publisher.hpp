#pragma once

#include "candle_types.hpp"
#include <chrono>
#include <mutex>
#include <string>
#include <vector>

struct __natsConnection;
struct __jsCtx;

namespace candle {

struct JetStreamConfig {
  std::string url;
  std::string stream;
  std::string subject_root;
  uint64_t chain_id{501};
  std::chrono::milliseconds publish_timeout{500};
};

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

/// JetStream publisher that serializes candles to protobuf and writes to NATS.
class JetStreamPublisher : public CandlePublisher {
public:
  explicit JetStreamPublisher(const JetStreamConfig &config);
  ~JetStreamPublisher() override;

  void publish(const std::string &pair_id, WindowSize window,
               const Candle &candle) override;

private:
  std::string window_label(WindowSize window) const;
  std::string build_subject(const std::string &pair_id, WindowSize window) const;
  std::string sanitize_token(const std::string &token) const;

  JetStreamConfig config_;
  __natsConnection *conn_{nullptr};
  __jsCtx *js_{nullptr};
  mutable std::mutex mutex_;
};

} // namespace candle
