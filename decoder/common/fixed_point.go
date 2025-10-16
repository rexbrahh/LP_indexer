package common

import (
	"fmt"
	"math"
	"math/big"
)

// Q64x64 represents a 128-bit fixed-point number with 64 bits for the integer and 64 bits for the fractional part
// This is used for sqrt price calculations in CLMM pools like Orca Whirlpools
const (
	Q64Shift = 64
)

var (
	Q64One = new(big.Int).Lsh(big.NewInt(1), Q64Shift)
)

// SqrtPriceQ64ToFloat converts a Q64.64 sqrt price to a float64 price
// Formula: price = (sqrt_price / 2^64)^2
func SqrtPriceQ64ToFloat(sqrtPriceQ64Str string) (float64, error) {
	sqrtPrice := new(big.Int)
	_, ok := sqrtPrice.SetString(sqrtPriceQ64Str, 10)
	if !ok {
		return 0, fmt.Errorf("invalid sqrt price string: %s", sqrtPriceQ64Str)
	}

	// Convert to big.Float for precision
	sqrtPriceFloat := new(big.Float).SetInt(sqrtPrice)

	// Divide by 2^64
	divisor := new(big.Float).SetInt(Q64One)
	sqrtPriceFloat.Quo(sqrtPriceFloat, divisor)

	// Square to get the actual price
	sqrtPriceFloat.Mul(sqrtPriceFloat, sqrtPriceFloat)

	// Convert to float64
	price, _ := sqrtPriceFloat.Float64()
	return price, nil
}

// FloatToSqrtPriceQ64 converts a float64 price to a Q64.64 sqrt price string
// Formula: sqrt_price_q64 = sqrt(price) * 2^64
func FloatToSqrtPriceQ64(price float64) string {
	sqrtPrice := math.Sqrt(price)
	sqrtPriceQ64 := new(big.Float).SetFloat64(sqrtPrice)

	// Multiply by 2^64
	multiplier := new(big.Float).SetInt(Q64One)
	sqrtPriceQ64.Mul(sqrtPriceQ64, multiplier)

	// Convert to big.Int
	result := new(big.Int)
	sqrtPriceQ64.Int(result)

	return result.String()
}

// TickIndexToSqrtPrice converts a tick index to a sqrt price
// Formula: sqrt_price = 1.0001^(tick_index / 2) * 2^64
func TickIndexToSqrtPrice(tickIndex int32) string {
	// Base for tick calculation: 1.0001
	base := 1.0001

	// Calculate sqrt price
	exponent := float64(tickIndex) / 2.0
	sqrtPrice := math.Pow(base, exponent)

	// Convert to Q64.64
	sqrtPriceQ64 := new(big.Float).SetFloat64(sqrtPrice)
	multiplier := new(big.Float).SetInt(Q64One)
	sqrtPriceQ64.Mul(sqrtPriceQ64, multiplier)

	result := new(big.Int)
	sqrtPriceQ64.Int(result)

	return result.String()
}

// SqrtPriceToTickIndex converts a sqrt price to the nearest tick index
// Formula: tick_index = floor(log_1.0001(sqrt_price / 2^64) * 2)
func SqrtPriceToTickIndex(sqrtPriceQ64Str string) (int32, error) {
	sqrtPrice := new(big.Int)
	_, ok := sqrtPrice.SetString(sqrtPriceQ64Str, 10)
	if !ok {
		return 0, fmt.Errorf("invalid sqrt price string: %s", sqrtPriceQ64Str)
	}

	// Convert to float64
	sqrtPriceFloat := new(big.Float).SetInt(sqrtPrice)
	divisor := new(big.Float).SetInt(Q64One)
	sqrtPriceFloat.Quo(sqrtPriceFloat, divisor)

	price, _ := sqrtPriceFloat.Float64()

	// Calculate tick index: log_1.0001(price) * 2
	// log_1.0001(x) = ln(x) / ln(1.0001)
	logBase := math.Log(1.0001)
	tickIndex := math.Floor(math.Log(price) / logBase * 2.0)

	return int32(tickIndex), nil
}

// ScaleAmount scales a raw token amount by its decimal places
func ScaleAmount(amount uint64, decimals uint8) float64 {
	divisor := math.Pow(10, float64(decimals))
	return float64(amount) / divisor
}

// UnscaleAmount converts a scaled amount back to raw token units
func UnscaleAmount(amount float64, decimals uint8) uint64 {
	multiplier := math.Pow(10, float64(decimals))
	return uint64(amount * multiplier)
}

// CalculatePriceFromAmounts calculates the effective price from swap amounts
func CalculatePriceFromAmounts(amountA, amountB uint64, decimalsA, decimalsB uint8) float64 {
	if amountA == 0 {
		return 0
	}

	scaledA := ScaleAmount(amountA, decimalsA)
	scaledB := ScaleAmount(amountB, decimalsB)

	// Price is quoted as B/A (quote/base)
	return scaledB / scaledA
}
