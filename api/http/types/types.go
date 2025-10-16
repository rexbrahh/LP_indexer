package types

import (
	"errors"
	"sort"
	"time"
)

// HealthResponse represents the shape of /healthz responses.
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// PoolResponse represents information about a liquidity pool.
type PoolResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseAsset  string `json:"base_asset"`
	QuoteAsset string `json:"quote_asset"`
}

// Candle represents OHLCV data for a pooled market.
type Candle struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// CandlesResponse represents the JSON response from the candle endpoint.
type CandlesResponse struct {
	PoolID    string   `json:"pool_id"`
	Timeframe string   `json:"timeframe"`
	Candles   []Candle `json:"candles"`
}

// ErrorResponse is a generic API error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ErrNotFound indicates missing resources.
var ErrNotFound = errors.New("not found")

// supportedTimeframes enumerates the accepted candle granularities.
var supportedTimeframes = map[string]struct{}{
	"1m": {},
	"5m": {},
	"1h": {},
	"1d": {},
}

// ValidateTimeframe ensures the provided timeframe matches supported values.
func ValidateTimeframe(tf string) error {
	if tf == "" {
		return errors.New("missing timeframe")
	}
	if _, ok := supportedTimeframes[tf]; !ok {
		return errors.New("invalid timeframe")
	}
	return nil
}

// SupportedTimeframes returns a stable list of accepted timeframes.
func SupportedTimeframes() []string {
	keys := make([]string, 0, len(supportedTimeframes))
	for k := range supportedTimeframes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
