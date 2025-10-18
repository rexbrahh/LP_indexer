#include "publisher.hpp"
#include "candle_worker.hpp"
#include "dex/sol/v1/core.pb.h"

#include <cctype>
#include <limits>
#include <sstream>
#include <stdexcept>
#include <utility>

#include <nats.h>

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

namespace {

std::string windowToString(WindowSize window) {
  switch (window) {
  case WindowSize::SEC_1:
    return "1s";
  case WindowSize::MIN_1:
    return "1m";
  case WindowSize::MIN_5:
    return "5m";
  case WindowSize::MIN_15:
    return "15m";
  case WindowSize::HOUR_1:
    return "1h";
  case WindowSize::HOUR_4:
    return "4h";
  case WindowSize::DAY_1:
    return "1d";
  default:
    return "custom";
  }
}

uint64_t toUint64(int64_t value) {
  if (value < 0) {
    return 0;
  }
  return static_cast<uint64_t>(value);
}

std::string natsErrorMessage(natsStatus status) {
  const char *text = natsStatus_GetText(status);
  return text != nullptr ? std::string{text} : std::string{"unknown"};
}

} // namespace

JetStreamPublisher::JetStreamPublisher(const JetStreamConfig &config)
    : config_(config) {
  natsOptions *opts = nullptr;
  natsStatus status = natsOptions_Create(&opts);
  if (status != NATS_OK) {
    throw std::runtime_error("natsOptions_Create failed: " +
                             natsErrorMessage(status));
  }

  if (config_.url.empty()) {
    config_.url = "nats://127.0.0.1:4222";
  }

  status = natsOptions_SetURL(opts, config_.url.c_str());
  if (status != NATS_OK) {
    natsOptions_Destroy(opts);
    throw std::runtime_error("natsOptions_SetURL failed: " +
                             natsErrorMessage(status));
  }

  status = natsConnection_Connect(&conn_, opts);
  natsOptions_Destroy(opts);
  if (status != NATS_OK) {
    throw std::runtime_error("natsConnection_Connect failed: " +
                             natsErrorMessage(status));
  }

  status = natsConnection_JetStream(&js_, conn_, nullptr);
  if (status != NATS_OK) {
    natsConnection_Destroy(conn_);
    conn_ = nullptr;
    throw std::runtime_error("natsConnection_JetStream failed: " +
                             natsErrorMessage(status));
  }
}

JetStreamPublisher::~JetStreamPublisher() {
  if (js_ != nullptr) {
    jsCtx_Destroy(js_);
    js_ = nullptr;
  }
  if (conn_ != nullptr) {
    natsConnection_Close(conn_);
    natsConnection_Destroy(conn_);
    conn_ = nullptr;
  }
}

std::string JetStreamPublisher::window_label(WindowSize window) const {
  return windowToString(window);
}

std::string JetStreamPublisher::sanitize_token(const std::string &token) const {
  std::string sanitized;
  sanitized.reserve(token.size());
  for (char c : token) {
    if (std::isalnum(static_cast<unsigned char>(c)) || c == '-') {
      sanitized.push_back(c);
    } else {
      sanitized.push_back('_');
    }
  }
  return sanitized;
}

std::string JetStreamPublisher::build_subject(const std::string &pair_id,
                                              WindowSize window) const {
  std::ostringstream subject;
  subject << config_.subject_root << ".candle." << window_label(window) << '.'
          << sanitize_token(pair_id);
  return subject.str();
}

void JetStreamPublisher::publish(const std::string &pair_id, WindowSize window,
                                 const Candle &candle) {
  if (js_ == nullptr) {
    throw std::runtime_error("JetStream context not initialized");
  }

  dex::sol::v1::Candle proto_candle;
  proto_candle.set_chain_id(config_.chain_id);
  proto_candle.set_pair_id(pair_id);
  proto_candle.set_timeframe(window_label(window));
  proto_candle.set_window_start(candle.open_time);
  proto_candle.set_provisional(candle.provisional);
  proto_candle.set_is_correction(false);
  proto_candle.set_open_px_q32(candle.open);
  proto_candle.set_high_px_q32(candle.high);
  proto_candle.set_low_px_q32(candle.low);
  proto_candle.set_close_px_q32(candle.close);
  proto_candle.set_trades(candle.trades);

  auto *vol_base = proto_candle.mutable_vol_base();
  vol_base->set_hi(0);
  vol_base->set_lo(toUint64(candle.volume));

  auto *vol_quote = proto_candle.mutable_vol_quote();
  vol_quote->set_hi(0);
  vol_quote->set_lo(toUint64(candle.quote_volume));

  std::string payload;
  if (!proto_candle.SerializeToString(&payload)) {
    throw std::runtime_error("failed to serialize candle protobuf");
  }

  if (payload.size() > static_cast<size_t>(std::numeric_limits<int>::max())) {
    throw std::runtime_error("payload too large for js_Publish");
  }

  jsPubAck *ack = nullptr;
  jsErrCode err_code = static_cast<jsErrCode>(0);
  std::string subject = build_subject(pair_id, window);
  std::string msg_id = subject + ":" + std::to_string(candle.open_time);

  jsPubOptions opts;
  jsPubOptions_Init(&opts);
  if (!config_.stream.empty()) {
    opts.ExpectStream = config_.stream.c_str();
  }
  opts.MsgId = msg_id.c_str();
  if (config_.publish_timeout.count() > 0) {
    opts.MaxWait = config_.publish_timeout.count();
  }

  natsStatus status;
  {
    std::lock_guard<std::mutex> lock(mutex_);
    status = js_Publish(&ack, js_, subject.c_str(), payload.data(),
                        static_cast<int>(payload.size()), &opts, &err_code);
  }

  if (ack != nullptr) {
    jsPubAck_Destroy(ack);
  }

  if (status != NATS_OK) {
    throw std::runtime_error("js_Publish failed: " + natsErrorMessage(status) +
                             ", jsErrCode=" + std::to_string(err_code));
  }
}

} // namespace candle
