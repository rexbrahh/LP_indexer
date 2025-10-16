#include "candle_worker.hpp"
#include <algorithm>
#include <functional>
#include <iostream>
#include <stdexcept>

namespace candle {

// ============================================================================
// CandleWindow Implementation
// ============================================================================

void CandleWindow::update(uint64_t timestamp, FixedPrice price,
                          FixedPrice base_amount, FixedPrice quote_amount) {
    uint64_t window_start = get_window_start(timestamp);
    uint64_t window_end = window_start + static_cast<uint32_t>(window_size);

    std::lock_guard<std::mutex> lock(mutex);

    auto it = candles.find(window_start);
    if (it == candles.end()) {
        // Create new candle
        Candle new_candle;
        new_candle.open_time = window_start;
        new_candle.close_time = window_end;
        new_candle.open = price;
        new_candle.high = price;
        new_candle.low = price;
        new_candle.close = price;
        new_candle.volume = base_amount;
        new_candle.quote_volume = quote_amount;
        new_candle.trades = 1;

        candles[window_start] = new_candle;
    } else {
        // Update existing candle
        Candle& candle = it->second;

        // Update high/low
        if (price > candle.high) {
            candle.high = price;
        }
        if (price < candle.low) {
            candle.low = price;
        }

        // Update close (last trade)
        candle.close = price;

        // Accumulate volume
        candle.volume += base_amount;
        candle.quote_volume += quote_amount;
        candle.trades++;
    }
}

uint64_t CandleWindow::get_window_start(uint64_t timestamp) const {
    uint32_t window_seconds = static_cast<uint32_t>(window_size);
    return (timestamp / window_seconds) * window_seconds;
}

// ============================================================================
// Shard Implementation
// ============================================================================

std::vector<std::shared_ptr<CandleWindow>>&
Shard::get_or_create_windows(const std::string& pair_id) {
    std::lock_guard<std::mutex> lock(mutex);

    auto it = windows.find(pair_id);
    if (it != windows.end()) {
        return it->second;
    }

    // Create windows for all supported time granularities
    std::vector<std::shared_ptr<CandleWindow>> new_windows;
    new_windows.reserve(6);

    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::MIN_1, pair_id));
    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::MIN_5, pair_id));
    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::MIN_15, pair_id));
    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::HOUR_1, pair_id));
    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::HOUR_4, pair_id));
    new_windows.push_back(std::make_shared<CandleWindow>(WindowSize::DAY_1, pair_id));

    windows[pair_id] = std::move(new_windows);
    return windows[pair_id];
}

void Shard::process_trade(const std::string& pair_id, uint64_t timestamp,
                          FixedPrice price, FixedPrice base_amount,
                          FixedPrice quote_amount) {
    auto& pair_windows = get_or_create_windows(pair_id);

    // Update all windows for this pair
    for (auto& window : pair_windows) {
        window->update(timestamp, price, base_amount, quote_amount);
    }
}

// ============================================================================
// CandleWorker Implementation
// ============================================================================

CandleWorker::CandleWorker(uint32_t num_shards)
    : num_shards_(num_shards), running_(false) {
    if (num_shards_ == 0) {
        throw std::invalid_argument("num_shards must be > 0");
    }
    init_shards();
}

CandleWorker::~CandleWorker() {
    stop();
}

void CandleWorker::init_shards() {
    shards_.reserve(num_shards_);
    for (uint32_t i = 0; i < num_shards_; ++i) {
        shards_.push_back(std::make_unique<Shard>(i));
    }
}

void CandleWorker::start() {
    bool expected = false;
    if (!running_.compare_exchange_strong(expected, true)) {
        return; // Already running
    }

    // TODO: Initialize worker thread pool for processing queued trades
    // For now, processing is synchronous in on_trade()
    std::cout << "CandleWorker started with " << num_shards_ << " shards\n";
}

void CandleWorker::stop() {
    bool expected = true;
    if (!running_.compare_exchange_strong(expected, false)) {
        return; // Already stopped
    }

    // TODO: Shutdown worker threads gracefully
    for (auto& thread : worker_threads_) {
        if (thread.joinable()) {
            thread.join();
        }
    }
    worker_threads_.clear();

    std::cout << "CandleWorker stopped\n";
}

void CandleWorker::on_trade(const std::string& pair_id, uint64_t timestamp,
                            FixedPrice price, FixedPrice base_amount,
                            FixedPrice quote_amount) {
    if (!running_) {
        return; // Worker not running
    }

    uint32_t shard_idx = get_shard_for_pair(pair_id);
    auto& shard = shards_[shard_idx];

    // Process trade synchronously for now
    // TODO: Queue to worker thread pool for async processing
    shard->process_trade(pair_id, timestamp, price, base_amount, quote_amount);
}

void CandleWorker::emit_candle(const std::string& pair_id,
                               WindowSize window_size, const Candle& candle) {
    // STUB: Provisional emit - no NATS integration yet
    // This will be called when we detect a window has closed
    // For now, just log to stdout for debugging

    std::cout << "[EMIT] pair=" << pair_id
              << " window=" << static_cast<uint32_t>(window_size)
              << " open_time=" << candle.open_time
              << " close_time=" << candle.close_time
              << " open=" << candle.open
              << " high=" << candle.high
              << " low=" << candle.low
              << " close=" << candle.close
              << " volume=" << candle.volume
              << " quote_volume=" << candle.quote_volume
              << " trades=" << candle.trades
              << "\n";

    // TODO: Serialize candle to protobuf and publish to NATS JetStream
    // Subject format: dex.sol.candles.{pair_id}.{window_size}
    // Msg-Id format: "501:{slot}:{sig}:{index}" for exactly-once semantics
}

uint32_t CandleWorker::get_shard_for_pair(const std::string& pair_id) const {
    return hash_pair_id(pair_id) % num_shards_;
}

uint32_t CandleWorker::hash_pair_id(const std::string& pair_id) const {
    // Simple FNV-1a hash for consistent shard assignment
    uint32_t hash = 2166136261u;
    for (char c : pair_id) {
        hash ^= static_cast<uint32_t>(c);
        hash *= 16777619u;
    }
    return hash;
}

} // namespace candle
