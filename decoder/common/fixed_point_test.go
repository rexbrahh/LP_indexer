package common

import (
	"math"
	"testing"
)

func TestSqrtPriceQ64ToFloat(t *testing.T) {
	tests := []struct {
		name          string
		sqrtPriceQ64  string
		expectedPrice float64
		tolerance     float64
	}{
		{
			name:          "price_1",
			sqrtPriceQ64:  FloatToSqrtPriceQ64(1.0),
			expectedPrice: 1.0,
			tolerance:     0.0001,
		},
		{
			name:          "price_100",
			sqrtPriceQ64:  FloatToSqrtPriceQ64(100.0),
			expectedPrice: 100.0,
			tolerance:     0.01,
		},
		{
			name:          "price_180",
			sqrtPriceQ64:  FloatToSqrtPriceQ64(180.0),
			expectedPrice: 180.0,
			tolerance:     0.01,
		},
		{
			name:          "price_0.5",
			sqrtPriceQ64:  FloatToSqrtPriceQ64(0.5),
			expectedPrice: 0.5,
			tolerance:     0.0001,
		},
		{
			name:          "price_10000",
			sqrtPriceQ64:  FloatToSqrtPriceQ64(10000.0),
			expectedPrice: 10000.0,
			tolerance:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := SqrtPriceQ64ToFloat(tt.sqrtPriceQ64)
			if err != nil {
				t.Fatalf("SqrtPriceQ64ToFloat failed: %v", err)
			}

			if math.Abs(price-tt.expectedPrice) > tt.tolerance {
				t.Errorf("Price mismatch: got %f, want %f (tolerance %f)", price, tt.expectedPrice, tt.tolerance)
			}
		})
	}
}

func TestFloatToSqrtPriceQ64(t *testing.T) {
	tests := []struct {
		name  string
		price float64
	}{
		{"price_1", 1.0},
		{"price_100", 100.0},
		{"price_180", 180.0},
		{"price_0.5", 0.5},
		{"price_10000", 10000.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to sqrt price Q64
			sqrtPriceQ64 := FloatToSqrtPriceQ64(tt.price)

			// Convert back to float
			price, err := SqrtPriceQ64ToFloat(sqrtPriceQ64)
			if err != nil {
				t.Fatalf("Round-trip conversion failed: %v", err)
			}

			// Should be very close to original
			if math.Abs(price-tt.price) > 0.01 {
				t.Errorf("Round-trip price mismatch: got %f, want %f", price, tt.price)
			}
		})
	}
}

func TestTickIndexToSqrtPrice(t *testing.T) {
	tests := []struct {
		name      string
		tickIndex int32
	}{
		{"tick_0", 0},
		{"tick_1000", 1000},
		{"tick_-1000", -1000},
		{"tick_50000", 50000},
		{"tick_-50000", -50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert tick to sqrt price
			sqrtPriceQ64 := TickIndexToSqrtPrice(tt.tickIndex)

			// Convert back to tick
			tickIndex, err := SqrtPriceToTickIndex(sqrtPriceQ64)
			if err != nil {
				t.Fatalf("Round-trip tick conversion failed: %v", err)
			}

			// Should be very close (within a few ticks due to rounding)
			if abs32(tickIndex-tt.tickIndex) > 2 {
				t.Errorf("Round-trip tick mismatch: got %d, want %d", tickIndex, tt.tickIndex)
			}
		})
	}
}

func TestScaleAmount(t *testing.T) {
	tests := []struct {
		name           string
		amount         uint64
		decimals       uint8
		expectedScaled float64
	}{
		{
			name:           "1_SOL",
			amount:         1000000000,
			decimals:       9,
			expectedScaled: 1.0,
		},
		{
			name:           "1_USDC",
			amount:         1000000,
			decimals:       6,
			expectedScaled: 1.0,
		},
		{
			name:           "123.456789_USDC",
			amount:         123456789,
			decimals:       6,
			expectedScaled: 123.456789,
		},
		{
			name:           "0.123456789_SOL",
			amount:         123456789,
			decimals:       9,
			expectedScaled: 0.123456789,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaled := ScaleAmount(tt.amount, tt.decimals)

			if math.Abs(scaled-tt.expectedScaled) > 0.0000001 {
				t.Errorf("ScaleAmount mismatch: got %f, want %f", scaled, tt.expectedScaled)
			}
		})
	}
}

func TestUnscaleAmount(t *testing.T) {
	tests := []struct {
		name             string
		amount           float64
		decimals         uint8
		expectedUnscaled uint64
	}{
		{
			name:             "1_SOL",
			amount:           1.0,
			decimals:         9,
			expectedUnscaled: 1000000000,
		},
		{
			name:             "1_USDC",
			amount:           1.0,
			decimals:         6,
			expectedUnscaled: 1000000,
		},
		{
			name:             "123.456789_USDC",
			amount:           123.456789,
			decimals:         6,
			expectedUnscaled: 123456789,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unscaled := UnscaleAmount(tt.amount, tt.decimals)

			if unscaled != tt.expectedUnscaled {
				t.Errorf("UnscaleAmount mismatch: got %d, want %d", unscaled, tt.expectedUnscaled)
			}
		})
	}
}

func TestCalculatePriceFromAmounts(t *testing.T) {
	tests := []struct {
		name          string
		amountA       uint64
		amountB       uint64
		decimalsA     uint8
		decimalsB     uint8
		expectedPrice float64
		tolerance     float64
	}{
		{
			name:          "1_SOL_to_180_USDC",
			amountA:       1000000000, // 1 SOL
			amountB:       180000000,  // 180 USDC
			decimalsA:     9,
			decimalsB:     6,
			expectedPrice: 180.0,
			tolerance:     0.01,
		},
		{
			name:          "1000_USDC_to_999_USDT",
			amountA:       1000000000, // 1000 USDC
			amountB:       999000000,  // 999 USDT
			decimalsA:     6,
			decimalsB:     6,
			expectedPrice: 0.999,
			tolerance:     0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := CalculatePriceFromAmounts(tt.amountA, tt.amountB, tt.decimalsA, tt.decimalsB)

			if math.Abs(price-tt.expectedPrice) > tt.tolerance {
				t.Errorf("CalculatePriceFromAmounts mismatch: got %f, want %f", price, tt.expectedPrice)
			}
		})
	}
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
