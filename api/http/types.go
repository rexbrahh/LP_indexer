package main

import apitypes "github.com/rexbrahh/lp-indexer/api/http/types"

type (
	// HealthResponse aliases the shared type for package-local convenience.
	HealthResponse = apitypes.HealthResponse
	// PoolResponse aliases the shared pool payload.
	PoolResponse = apitypes.PoolResponse
	// Candle aliases the OHLCV representation.
	Candle = apitypes.Candle
	// CandlesResponse aliases the candle response payload.
	CandlesResponse = apitypes.CandlesResponse
	// ErrorResponse aliases the generic error payload.
	ErrorResponse = apitypes.ErrorResponse
)

var (
	// ErrNotFound exposes the shared not-found error.
	ErrNotFound = apitypes.ErrNotFound
	// SupportedTimeframes exposes the list of allowed timeframes.
	SupportedTimeframes = apitypes.SupportedTimeframes
	// ValidateTimeframe exposes the timeframe validator.
	ValidateTimeframe = apitypes.ValidateTimeframe
)
