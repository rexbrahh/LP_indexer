#include "fixed_point.hpp"
#include <iomanip>
#include <sstream>
#include <cmath>

namespace candle {

// ============================================================================
// FixedPoint Methods
// ============================================================================

std::string FixedPoint::to_string() const {
    std::ostringstream oss;
    oss << std::fixed << std::setprecision(9) << to_double();
    return oss.str();
}

// ============================================================================
// Multiplication and Division
// ============================================================================

FixedPoint fp_multiply(const FixedPoint& a, const FixedPoint& b) {
    // Multiply using 128-bit intermediate to avoid overflow
    // Result = (a.raw * b.raw) >> 32

    detail::Int128 product = detail::multiply_64x64_to_128(a.raw(), b.raw());

    // Shift right by FRACTIONAL_BITS to get the final Q32.32 result
    detail::Int128 shifted = detail::shift_right_128(product, FRACTIONAL_BITS);

    if (!detail::fits_in_int64(shifted)) {
        throw std::overflow_error("Fixed-point multiplication overflow");
    }

    return FixedPoint(detail::to_int64(shifted));
}

FixedPoint fp_divide(const FixedPoint& a, const FixedPoint& b) {
    if (b.raw() == 0) {
        throw std::domain_error("Division by zero");
    }

    // Divide using 128-bit intermediate for precision
    // Result = (a.raw << 32) / b.raw

    // First, extend a.raw to 128-bit and shift left by FRACTIONAL_BITS
    detail::Int128 dividend;
    if (a.raw() >= 0) {
        dividend.high = static_cast<int64_t>(static_cast<uint64_t>(a.raw()) >> 32);
        dividend.low = static_cast<uint64_t>(a.raw()) << 32;
    } else {
        // Handle negative values
        int64_t abs_a = -a.raw();
        dividend.high = -(static_cast<int64_t>(static_cast<uint64_t>(abs_a) >> 32));
        dividend.low = static_cast<uint64_t>(abs_a) << 32;
        if (dividend.low != 0) {
            dividend.high--; // Adjust for two's complement
            dividend.low = ~dividend.low + 1;
        }
    }

    int64_t quotient = detail::divide_128_by_64(dividend, b.raw());
    return FixedPoint(quotient);
}

// ============================================================================
// Helper functions for 128-bit arithmetic
// ============================================================================

namespace detail {

Int128 multiply_64x64_to_128(int64_t a, int64_t b) {
    // Handle sign
    bool negative = (a < 0) != (b < 0);

    // Work with absolute values
    uint64_t abs_a = (a < 0) ? -static_cast<uint64_t>(a) : static_cast<uint64_t>(a);
    uint64_t abs_b = (b < 0) ? -static_cast<uint64_t>(b) : static_cast<uint64_t>(b);

    // Split into 32-bit parts
    uint64_t a_lo = abs_a & 0xFFFFFFFF;
    uint64_t a_hi = abs_a >> 32;
    uint64_t b_lo = abs_b & 0xFFFFFFFF;
    uint64_t b_hi = abs_b >> 32;

    // Multiply parts
    uint64_t p0 = a_lo * b_lo;
    uint64_t p1 = a_lo * b_hi;
    uint64_t p2 = a_hi * b_lo;
    uint64_t p3 = a_hi * b_hi;

    // Combine (schoolbook multiplication)
    uint64_t middle = p1 + (p0 >> 32) + (p2 & 0xFFFFFFFF);

    uint64_t low = (middle << 32) | (p0 & 0xFFFFFFFF);
    uint64_t high = p3 + (p2 >> 32) + (middle >> 32);

    Int128 result;
    result.low = low;
    result.high = static_cast<int64_t>(high);

    // Apply sign
    if (negative) {
        // Two's complement negation
        result.low = ~result.low + 1;
        result.high = ~result.high;
        if (result.low == 0) {
            result.high++;
        }
    }

    return result;
}

int64_t divide_128_by_64(const Int128& dividend, int64_t divisor) {
    if (divisor == 0) {
        throw std::domain_error("Division by zero");
    }

    // Handle sign
    bool negative = (dividend.high < 0) != (divisor < 0);

    // Work with absolute values
    Int128 abs_dividend = dividend;
    uint64_t abs_divisor;

    if (dividend.high < 0) {
        // Negate 128-bit value
        abs_dividend.low = ~abs_dividend.low + 1;
        abs_dividend.high = ~abs_dividend.high;
        if (abs_dividend.low == 0) {
            abs_dividend.high++;
        }
    }

    if (divisor < 0) {
        abs_divisor = -static_cast<uint64_t>(divisor);
    } else {
        abs_divisor = static_cast<uint64_t>(divisor);
    }

    // Check if dividend fits in 64 bits for simple division
    if (abs_dividend.high == 0) {
        uint64_t quotient = abs_dividend.low / abs_divisor;
        if (quotient > static_cast<uint64_t>(INT64_MAX)) {
            throw std::overflow_error("Division result overflow");
        }
        int64_t result = static_cast<int64_t>(quotient);
        return negative ? -result : result;
    }

    // Perform long division for 128-bit / 64-bit
    // This is a simplified implementation assuming the result fits in 64 bits

    uint64_t quotient = 0;
    Int128 remainder = abs_dividend;

    // Find the highest bit set in divisor
    int shift = 63;
    while (shift >= 0 && !((abs_divisor >> shift) & 1)) {
        shift--;
    }

    // Perform division bit-by-bit (can be optimized)
    for (int i = 63; i >= 0; i--) {
        // Check if remainder >= (divisor << i)
        uint64_t divisor_shifted_high = (i >= 64) ? abs_divisor << (i - 64) : 0;
        uint64_t divisor_shifted_low = (i < 64) ? abs_divisor << i : 0;

        // Compare
        bool can_subtract = false;
        if (static_cast<uint64_t>(remainder.high) > divisor_shifted_high) {
            can_subtract = true;
        } else if (static_cast<uint64_t>(remainder.high) == divisor_shifted_high &&
                   remainder.low >= divisor_shifted_low) {
            can_subtract = true;
        }

        if (can_subtract) {
            quotient |= (1ULL << i);
            // Subtract
            if (remainder.low < divisor_shifted_low) {
                remainder.high--;
            }
            remainder.low -= divisor_shifted_low;
            remainder.high -= divisor_shifted_high;
        }
    }

    if (quotient > static_cast<uint64_t>(INT64_MAX)) {
        throw std::overflow_error("Division result overflow");
    }

    int64_t result = static_cast<int64_t>(quotient);
    return negative ? -result : result;
}

Int128 shift_right_128(const Int128& value, int n) {
    if (n <= 0 || n >= 64) {
        throw std::invalid_argument("Shift amount must be between 1 and 63");
    }

    Int128 result;
    // Arithmetic right shift (preserve sign)
    result.high = value.high >> n;
    result.low = (static_cast<uint64_t>(value.high) << (64 - n)) |
                 (value.low >> n);

    return result;
}

Int128 shift_left_128(const Int128& value, int n) {
    if (n <= 0 || n >= 64) {
        throw std::invalid_argument("Shift amount must be between 1 and 63");
    }

    Int128 result;
    result.high = (value.high << n) |
                  static_cast<int64_t>(value.low >> (64 - n));
    result.low = value.low << n;

    return result;
}

bool fits_in_int64(const Int128& value) {
    // Check if value fits in int64_t range
    // For positive: high must be 0, and low must fit in positive int64_t
    // For negative: high must be -1, and low must represent negative int64_t

    if (value.high == 0) {
        return value.low <= static_cast<uint64_t>(INT64_MAX);
    } else if (value.high == -1) {
        return value.low >= static_cast<uint64_t>(INT64_MIN);
    }
    return false;
}

int64_t to_int64(const Int128& value) {
    if (!fits_in_int64(value)) {
        throw std::overflow_error("128-bit value does not fit in int64_t");
    }
    return static_cast<int64_t>(value.low);
}

} // namespace detail

} // namespace candle
