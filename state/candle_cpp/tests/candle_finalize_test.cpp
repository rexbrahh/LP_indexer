#include <gtest/gtest.h>
#include <thread>
#include <chrono>
#include "candle_worker.hpp"
#include "fixed_point.hpp"

using namespace candle;

// ============================================================================
// Candle Finalization Tests
// ============================================================================

TEST(CandleFinalizeTest, ProvisionialFlagInitiallyTrue) {
    Candle candle;
    EXPECT_TRUE(candle.provisional);
}

TEST(CandleFinalizeTest, WatermarkUpdatesOnTrade) {
    CandleWindow window(WindowSize::MIN_1, "SOL/USDC");

    uint64_t timestamp1 = 1700000060;
    uint64_t timestamp2 = 1700000065;

    FixedPrice price = FixedPoint::from_double(100.0).raw();
    FixedPrice volume = FixedPoint::from_double(10.0).raw();

    window.update(timestamp1, price, volume, volume);
    EXPECT_EQ(window.last_trade_time, timestamp1);

    window.update(timestamp2, price, volume, volume);
    EXPECT_EQ(window.last_trade_time, timestamp2);
}

TEST(CandleFinalizeTest, FinalizeOldCandlesFlipsProvisionalFlag) {
    CandleWindow window(WindowSize::MIN_1, "SOL/USDC");

    // Create a candle at timestamp 1700000060 (window: 1700000040-1700000100)
    uint64_t timestamp = 1700000060;
    FixedPrice price = FixedPoint::from_double(100.0).raw();
    FixedPrice volume = FixedPoint::from_double(10.0).raw();

    window.update(timestamp, price, volume, volume);

    // Check candle exists and is provisional
    {
        std::lock_guard<std::mutex> lock(window.mutex);
        uint64_t window_start = window.get_window_start(timestamp);
        ASSERT_TRUE(window.candles.find(window_start) != window.candles.end());
        EXPECT_TRUE(window.candles[window_start].provisional);
    }

    // Finalize with watermark after window close time
    uint64_t watermark = 1700000100;
    auto finalized = window.finalize_old_candles(watermark);

    // Should have finalized one candle
    ASSERT_EQ(finalized.size(), 1);
    EXPECT_FALSE(finalized[0].provisional);
    EXPECT_EQ(finalized[0].open_time, 1700000040);
    EXPECT_EQ(finalized[0].close_time, 1700000100);

    // Window should be cleared after finalization
    {
        std::lock_guard<std::mutex> lock(window.mutex);
        uint64_t window_start = window.get_window_start(timestamp);
        EXPECT_TRUE(window.candles.find(window_start) == window.candles.end());
    }
}

TEST(CandleFinalizeTest, DoesNotFinalizeCurrentWindow) {
    CandleWindow window(WindowSize::MIN_1, "SOL/USDC");

    // Create a candle at timestamp 1700000060 (window: 1700000040-1700000100)
    uint64_t timestamp = 1700000060;
    FixedPrice price = FixedPoint::from_double(100.0).raw();
    FixedPrice volume = FixedPoint::from_double(10.0).raw();

    window.update(timestamp, price, volume, volume);

    // Finalize with watermark BEFORE window close time
    uint64_t watermark = 1700000080;
    auto finalized = window.finalize_old_candles(watermark);

    // Should NOT finalize (window still open)
    EXPECT_EQ(finalized.size(), 0);

    // Candle should still exist
    {
        std::lock_guard<std::mutex> lock(window.mutex);
        uint64_t window_start = window.get_window_start(timestamp);
        ASSERT_TRUE(window.candles.find(window_start) != window.candles.end());
        EXPECT_TRUE(window.candles[window_start].provisional);
    }
}

TEST(CandleFinalizeTest, WorkerEmitsFinalizedCandles) {
    CandleWorker worker(4);
    worker.start();

    // Send trades
    uint64_t base_time = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count() - 120;

    FixedPrice price = FixedPoint::from_double(100.0).raw();
    FixedPrice volume = FixedPoint::from_double(10.0).raw();

    // Trade in a window that should close soon
    worker.on_trade("SOL/USDC", base_time, price, volume, volume);

    // Wait for timing wheel to finalize (1s tick + buffer)
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // Check emitted candles
    auto emitted = worker.get_emitted_candles();

    // Should have emitted at least the 1m window candle
    // (may have more from other window sizes)
    EXPECT_GT(emitted.size(), 0);

    // Find the 1m candle
    bool found_1m_candle = false;
    for (const auto& candle : emitted) {
        if (candle.open_time <= base_time && candle.close_time > base_time) {
            EXPECT_FALSE(candle.provisional);
            EXPECT_EQ(candle.trades, 1);
            found_1m_candle = true;
        }
    }

    EXPECT_TRUE(found_1m_candle);

    worker.stop();
}

TEST(CandleFinalizeTest, MultipleWindowsFinalized) {
    CandleWindow window(WindowSize::MIN_1, "SOL/USDC");

    // Create multiple candles in different windows
    uint64_t base_time = 1700000000;
    FixedPrice price = FixedPoint::from_double(100.0).raw();
    FixedPrice volume = FixedPoint::from_double(10.0).raw();

    // Window 1: 1700000000-1700000060
    window.update(base_time + 10, price, volume, volume);

    // Window 2: 1700000060-1700000120
    window.update(base_time + 70, price, volume, volume);

    // Window 3: 1700000120-1700000180
    window.update(base_time + 130, price, volume, volume);

    // Finalize windows 1 and 2
    uint64_t watermark = base_time + 120;
    auto finalized = window.finalize_old_candles(watermark);

    // Should finalize 2 windows
    ASSERT_EQ(finalized.size(), 2);

    for (const auto& candle : finalized) {
        EXPECT_FALSE(candle.provisional);
    }

    // Window 3 should still exist
    {
        std::lock_guard<std::mutex> lock(window.mutex);
        EXPECT_EQ(window.candles.size(), 1);
    }
}

// ============================================================================
// Main
// ============================================================================

int main(int argc, char** argv) {
    ::testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}
