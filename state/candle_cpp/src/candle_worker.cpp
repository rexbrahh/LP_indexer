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

    // Update watermark
    if (timestamp > last_trade_time) {
        last_trade_time = timestamp;
    }

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
        new_candle.provisional = true;

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

std::vector<Candle> CandleWindow::finalize_old_candles(uint64_t watermark) {
    std::lock_guard<std::mutex> lock(mutex);
    std::vector<Candle> finalized;

    // Find all candles whose close_time is before the watermark
    auto it = candles.begin();
    while (it != candles.end()) {
        Candle& candle = it->second;

        // Finalize if window has closed and candle is still provisional
        if (candle.close_time <= watermark && candle.provisional) {
            candle.provisional = false;
            finalized.push_back(candle);
            // Remove the finalized candle to free memory
            it = candles.erase(it);
        } else {
            ++it;
        }
    }

    return finalized;
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

    // Start timing wheel for finalization
    finalize_thread_ = std::thread([this]() { finalize_loop(); });

    std::cout << "CandleWorker started with " << num_shards_ << " shards\n";
}

void CandleWorker::stop() {
    bool expected = true;
    if (!running_.compare_exchange_strong(expected, false)) {
        return; // Already stopped
    }

    // Stop timing wheel
    if (finalize_thread_.joinable()) {
        finalize_thread_.join();
    }

    // Shutdown worker threads gracefully
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
    // Enqueue to in-memory sink for now (provisional)
    {
        std::lock_guard<std::mutex> lock(emitted_mutex_);
        emitted_candles_.push_back(candle);
    }

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
              << " provisional=" << candle.provisional
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

std::vector<Candle> CandleWorker::get_emitted_candles() const {
    std::lock_guard<std::mutex> lock(emitted_mutex_);
    return emitted_candles_;
}

void CandleWorker::finalize_loop() {
    // Timing wheel: periodically check for candles to finalize
    // For 1m windows, check every 1 second
    while (running_) {
        std::this_thread::sleep_for(std::chrono::seconds(1));

        // Get current time as watermark
        auto now = std::chrono::system_clock::now();
        uint64_t watermark = std::chrono::duration_cast<std::chrono::seconds>(
            now.time_since_epoch()).count();

        // Iterate through all shards and windows to finalize old candles
        for (auto& shard : shards_) {
            std::lock_guard<std::mutex> shard_lock(shard->mutex);

            for (auto& [pair_id, windows] : shard->windows) {
                for (auto& window : windows) {
                    // Check if this window has a last_trade_time
                    // Only finalize windows where no new trades arrive
                    uint64_t last_trade = 0;
                    {
                        std::lock_guard<std::mutex> win_lock(window->mutex);
                        last_trade = window->last_trade_time;
                    }

                    // If no trades yet, skip
                    if (last_trade == 0) {
                        continue;
                    }

                    // Finalize candles that closed before the watermark
                    auto finalized = window->finalize_old_candles(watermark);

                    // Emit finalized candles
                    for (const auto& candle : finalized) {
                        emit_candle(pair_id, window->window_size, candle);
                    }
                }
            }
        }
    }
}

} // namespace candle
