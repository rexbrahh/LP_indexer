#include <gtest/gtest.h>
#include "fixed_point.hpp"

using namespace candle;

// ============================================================================
// Basic Construction and Conversion Tests
// ============================================================================

TEST(FixedPointTest, DefaultConstructor) {
    FixedPoint fp;
    EXPECT_EQ(fp.raw(), 0);
    EXPECT_EQ(fp.to_int(), 0);
    EXPECT_DOUBLE_EQ(fp.to_double(), 0.0);
}

TEST(FixedPointTest, FromInt) {
    FixedPoint fp = FixedPoint::from_int(42);
    EXPECT_EQ(fp.to_int(), 42);
    EXPECT_DOUBLE_EQ(fp.to_double(), 42.0);

    FixedPoint negative = FixedPoint::from_int(-100);
    EXPECT_EQ(negative.to_int(), -100);
    EXPECT_DOUBLE_EQ(negative.to_double(), -100.0);
}

TEST(FixedPointTest, FromDouble) {
    FixedPoint fp = FixedPoint::from_double(3.14159);
    EXPECT_NEAR(fp.to_double(), 3.14159, 1e-9);

    FixedPoint negative = FixedPoint::from_double(-2.71828);
    EXPECT_NEAR(negative.to_double(), -2.71828, 1e-9);
}

TEST(FixedPointTest, RawValue) {
    FixedPoint one = FixedPoint::from_int(1);
    EXPECT_EQ(one.raw(), FIXED_ONE);

    FixedPoint half = FixedPoint::from_double(0.5);
    EXPECT_EQ(half.raw(), FIXED_ONE / 2);
}

// ============================================================================
// Arithmetic Operations Tests
// ============================================================================

TEST(FixedPointTest, Addition) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(5);
    FixedPoint result = a + b;

    EXPECT_EQ(result.to_int(), 15);
    EXPECT_DOUBLE_EQ(result.to_double(), 15.0);
}

TEST(FixedPointTest, AdditionWithFractional) {
    FixedPoint a = FixedPoint::from_double(3.5);
    FixedPoint b = FixedPoint::from_double(2.25);
    FixedPoint result = a + b;

    EXPECT_NEAR(result.to_double(), 5.75, 1e-9);
}

TEST(FixedPointTest, Subtraction) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(3);
    FixedPoint result = a - b;

    EXPECT_EQ(result.to_int(), 7);
    EXPECT_DOUBLE_EQ(result.to_double(), 7.0);
}

TEST(FixedPointTest, SubtractionWithFractional) {
    FixedPoint a = FixedPoint::from_double(5.75);
    FixedPoint b = FixedPoint::from_double(2.25);
    FixedPoint result = a - b;

    EXPECT_NEAR(result.to_double(), 3.5, 1e-9);
}

TEST(FixedPointTest, UnaryMinus) {
    FixedPoint a = FixedPoint::from_double(3.14);
    FixedPoint neg = -a;

    EXPECT_NEAR(neg.to_double(), -3.14, 1e-9);
}

TEST(FixedPointTest, CompoundAddition) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(5);
    a += b;

    EXPECT_EQ(a.to_int(), 15);
}

TEST(FixedPointTest, CompoundSubtraction) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(3);
    a -= b;

    EXPECT_EQ(a.to_int(), 7);
}

// ============================================================================
// Comparison Tests
// ============================================================================

TEST(FixedPointTest, Equality) {
    FixedPoint a = FixedPoint::from_double(3.14);
    FixedPoint b = FixedPoint::from_double(3.14);
    FixedPoint c = FixedPoint::from_double(2.71);

    EXPECT_TRUE(a == b);
    EXPECT_FALSE(a == c);
    EXPECT_TRUE(a != c);
    EXPECT_FALSE(a != b);
}

TEST(FixedPointTest, Comparison) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(5);
    FixedPoint c = FixedPoint::from_int(10);

    EXPECT_TRUE(a > b);
    EXPECT_FALSE(b > a);
    EXPECT_TRUE(a >= b);
    EXPECT_TRUE(a >= c);

    EXPECT_TRUE(b < a);
    EXPECT_FALSE(a < b);
    EXPECT_TRUE(b <= a);
    EXPECT_TRUE(a <= c);
}

// ============================================================================
// Multiplication Tests (128-bit safety)
// ============================================================================

TEST(FixedPointTest, MultiplySimple) {
    FixedPoint a = FixedPoint::from_int(3);
    FixedPoint b = FixedPoint::from_int(4);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_EQ(result.to_int(), 12);
    EXPECT_DOUBLE_EQ(result.to_double(), 12.0);
}

TEST(FixedPointTest, MultiplyWithFractional) {
    FixedPoint a = FixedPoint::from_double(2.5);
    FixedPoint b = FixedPoint::from_double(4.0);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_NEAR(result.to_double(), 10.0, 1e-9);
}

TEST(FixedPointTest, MultiplyFractionalByFractional) {
    FixedPoint a = FixedPoint::from_double(1.5);
    FixedPoint b = FixedPoint::from_double(2.5);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_NEAR(result.to_double(), 3.75, 1e-9);
}

TEST(FixedPointTest, MultiplyNegative) {
    FixedPoint a = FixedPoint::from_int(-3);
    FixedPoint b = FixedPoint::from_int(4);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_EQ(result.to_int(), -12);

    FixedPoint c = FixedPoint::from_int(-3);
    FixedPoint d = FixedPoint::from_int(-4);
    FixedPoint result2 = fp_multiply(c, d);

    EXPECT_EQ(result2.to_int(), 12);
}

TEST(FixedPointTest, MultiplyLargeValues) {
    FixedPoint a = FixedPoint::from_int(1000000);
    FixedPoint b = FixedPoint::from_int(1000);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_EQ(result.to_int(), 1000000000);
}

TEST(FixedPointTest, MultiplyByZero) {
    FixedPoint a = FixedPoint::from_int(42);
    FixedPoint zero = FixedPoint::from_int(0);
    FixedPoint result = fp_multiply(a, zero);

    EXPECT_EQ(result.to_int(), 0);
}

// ============================================================================
// Division Tests (128-bit safety)
// ============================================================================

TEST(FixedPointTest, DivideSimple) {
    FixedPoint a = FixedPoint::from_int(12);
    FixedPoint b = FixedPoint::from_int(4);
    FixedPoint result = fp_divide(a, b);

    EXPECT_EQ(result.to_int(), 3);
    EXPECT_DOUBLE_EQ(result.to_double(), 3.0);
}

TEST(FixedPointTest, DivideWithFractional) {
    FixedPoint a = FixedPoint::from_int(10);
    FixedPoint b = FixedPoint::from_int(4);
    FixedPoint result = fp_divide(a, b);

    EXPECT_NEAR(result.to_double(), 2.5, 1e-9);
}

TEST(FixedPointTest, DivideFractionalByFractional) {
    FixedPoint a = FixedPoint::from_double(7.5);
    FixedPoint b = FixedPoint::from_double(2.5);
    FixedPoint result = fp_divide(a, b);

    EXPECT_NEAR(result.to_double(), 3.0, 1e-9);
}

TEST(FixedPointTest, DivideNegative) {
    FixedPoint a = FixedPoint::from_int(-12);
    FixedPoint b = FixedPoint::from_int(4);
    FixedPoint result = fp_divide(a, b);

    EXPECT_EQ(result.to_int(), -3);

    FixedPoint c = FixedPoint::from_int(-12);
    FixedPoint d = FixedPoint::from_int(-4);
    FixedPoint result2 = fp_divide(c, d);

    EXPECT_EQ(result2.to_int(), 3);
}

TEST(FixedPointTest, DivideByZero) {
    FixedPoint a = FixedPoint::from_int(42);
    FixedPoint zero = FixedPoint::from_int(0);

    EXPECT_THROW(fp_divide(a, zero), std::domain_error);
}

TEST(FixedPointTest, DivideLargeValues) {
    FixedPoint a = FixedPoint::from_int(1000000000);
    FixedPoint b = FixedPoint::from_int(1000);
    FixedPoint result = fp_divide(a, b);

    EXPECT_EQ(result.to_int(), 1000000);
}

// ============================================================================
// Edge Cases and Precision Tests
// ============================================================================

TEST(FixedPointTest, SmallFractionalValues) {
    FixedPoint a = FixedPoint::from_double(0.000001);
    FixedPoint b = FixedPoint::from_double(1000000.0);
    FixedPoint result = fp_multiply(a, b);

    EXPECT_NEAR(result.to_double(), 1.0, 1e-6);
}

TEST(FixedPointTest, PrecisionMaintenance) {
    // Test that we maintain precision through multiple operations
    FixedPoint a = FixedPoint::from_double(1.0 / 3.0);
    FixedPoint three = FixedPoint::from_int(3);
    FixedPoint result = fp_multiply(a, three);

    EXPECT_NEAR(result.to_double(), 1.0, 1e-9);
}

TEST(FixedPointTest, ToString) {
    FixedPoint a = FixedPoint::from_double(3.14159);
    std::string str = a.to_string();

    // Just verify it doesn't crash and produces a non-empty string
    EXPECT_FALSE(str.empty());
}

// ============================================================================
// 128-bit Helper Tests
// ============================================================================

TEST(Int128Test, Multiply64x64Simple) {
    using namespace detail;

    Int128 result = multiply_64x64_to_128(100, 200);
    EXPECT_EQ(result.high, 0);
    EXPECT_EQ(result.low, 20000ULL);
}

TEST(Int128Test, Multiply64x64Large) {
    using namespace detail;

    // Multiply two large numbers that would overflow 64-bit
    int64_t a = 1LL << 40;  // 2^40
    int64_t b = 1LL << 30;  // 2^30
    Int128 result = multiply_64x64_to_128(a, b);

    // Expected: 2^70, which requires more than 64 bits
    EXPECT_EQ(result.high, 1LL << 6);  // Upper 64 bits
    EXPECT_EQ(result.low, 0ULL);        // Lower 64 bits
}

TEST(Int128Test, Multiply64x64Negative) {
    using namespace detail;

    Int128 result = multiply_64x64_to_128(-100, 200);
    EXPECT_EQ(result.high, -1);
    EXPECT_EQ(result.low, static_cast<uint64_t>(-20000));
}

TEST(Int128Test, FitsInInt64) {
    using namespace detail;

    Int128 fits{0, 1000};
    EXPECT_TRUE(fits_in_int64(fits));

    Int128 too_large{1, 0};
    EXPECT_FALSE(fits_in_int64(too_large));

    Int128 negative{-1, static_cast<uint64_t>(-1000)};
    EXPECT_TRUE(fits_in_int64(negative));
}

// ============================================================================
// Main
// ============================================================================

int main(int argc, char** argv) {
    ::testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}
