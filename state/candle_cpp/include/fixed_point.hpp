#pragma once

#include <cstdint>
#include <stdexcept>
#include <string>

namespace candle {

/// Q32.32 fixed-point arithmetic utilities
/// Represents a fixed-point number with 32 bits for the integer part
/// and 32 bits for the fractional part.
///
/// Internal representation: int64_t where value = raw / 2^32
///
/// Example:
///   1.0 is represented as 0x0000000100000000 (4294967296)
///   0.5 is represented as 0x0000000080000000 (2147483648)

constexpr int32_t FRACTIONAL_BITS = 32;
constexpr int64_t FIXED_ONE = 1LL << FRACTIONAL_BITS; // 2^32

class FixedPoint {
public:
    /// Constructors
    constexpr FixedPoint() : raw_(0) {}
    constexpr explicit FixedPoint(int64_t raw_value) : raw_(raw_value) {}

    /// Create from integer
    static constexpr FixedPoint from_int(int64_t value) {
        return FixedPoint(value << FRACTIONAL_BITS);
    }

    /// Create from double (for convenience, use sparingly)
    static FixedPoint from_double(double value) {
        return FixedPoint(static_cast<int64_t>(value * FIXED_ONE));
    }

    /// Get raw value
    constexpr int64_t raw() const { return raw_; }

    /// Convert to integer (truncates fractional part)
    constexpr int64_t to_int() const { return raw_ >> FRACTIONAL_BITS; }

    /// Convert to double (for display/debugging)
    double to_double() const {
        return static_cast<double>(raw_) / FIXED_ONE;
    }

    /// Addition
    constexpr FixedPoint operator+(const FixedPoint& other) const {
        return FixedPoint(raw_ + other.raw_);
    }

    /// Subtraction
    constexpr FixedPoint operator-(const FixedPoint& other) const {
        return FixedPoint(raw_ - other.raw_);
    }

    /// Unary minus
    constexpr FixedPoint operator-() const {
        return FixedPoint(-raw_);
    }

    /// Compound addition
    FixedPoint& operator+=(const FixedPoint& other) {
        raw_ += other.raw_;
        return *this;
    }

    /// Compound subtraction
    FixedPoint& operator-=(const FixedPoint& other) {
        raw_ -= other.raw_;
        return *this;
    }

    /// Comparison operators
    constexpr bool operator==(const FixedPoint& other) const {
        return raw_ == other.raw_;
    }
    constexpr bool operator!=(const FixedPoint& other) const {
        return raw_ != other.raw_;
    }
    constexpr bool operator<(const FixedPoint& other) const {
        return raw_ < other.raw_;
    }
    constexpr bool operator<=(const FixedPoint& other) const {
        return raw_ <= other.raw_;
    }
    constexpr bool operator>(const FixedPoint& other) const {
        return raw_ > other.raw_;
    }
    constexpr bool operator>=(const FixedPoint& other) const {
        return raw_ >= other.raw_;
    }

    /// String representation (for debugging)
    std::string to_string() const;

private:
    int64_t raw_;
};

// ============================================================================
// Multiplication and Division with 128-bit intermediate safety
// ============================================================================

/// Multiply two Q32.32 fixed-point numbers using 128-bit intermediate
/// to prevent overflow during multiplication.
///
/// Algorithm:
///   (a * b) / 2^32 where intermediate is 128-bit
///
/// @throws std::overflow_error if result exceeds int64_t range
FixedPoint fp_multiply(const FixedPoint& a, const FixedPoint& b);

/// Divide two Q32.32 fixed-point numbers using 128-bit intermediate
/// to maintain precision and prevent overflow.
///
/// Algorithm:
///   (a * 2^32) / b where intermediate is 128-bit
///
/// @throws std::domain_error if b is zero
/// @throws std::overflow_error if result exceeds int64_t range
FixedPoint fp_divide(const FixedPoint& a, const FixedPoint& b);

// ============================================================================
// Helper functions for 128-bit arithmetic
// ============================================================================

namespace detail {

/// Represents a 128-bit signed integer (simulated with high/low parts)
struct Int128 {
    int64_t high; // Upper 64 bits (signed)
    uint64_t low; // Lower 64 bits (unsigned)

    constexpr Int128() : high(0), low(0) {}
    constexpr Int128(int64_t h, uint64_t l) : high(h), low(l) {}
};

/// Multiply two int64_t values into a 128-bit result
Int128 multiply_64x64_to_128(int64_t a, int64_t b);

/// Divide a 128-bit value by a 64-bit divisor, returning a 64-bit result
/// @throws std::domain_error if divisor is zero
/// @throws std::overflow_error if quotient exceeds int64_t range
int64_t divide_128_by_64(const Int128& dividend, int64_t divisor);

/// Shift right a 128-bit value by n bits (0 < n < 64)
Int128 shift_right_128(const Int128& value, int n);

/// Shift left a 128-bit value by n bits (0 < n < 64)
Int128 shift_left_128(const Int128& value, int n);

/// Check if a 128-bit value fits in int64_t
bool fits_in_int64(const Int128& value);

/// Convert 128-bit to int64_t (assumes it fits)
int64_t to_int64(const Int128& value);

} // namespace detail

} // namespace candle
