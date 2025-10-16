package common

import (
	"fmt"
)

// Priority order for canonical pair determination
// Higher priority tokens should be the quote (second token in pair)
var quotePriority = map[string]int{
	"USDC":   100, // Highest priority quote
	"USDT":   90,
	"SOL":    80,
	"WSOL":   75,
	"ETH":    70,
	"WETH":   65,
	"BTC":    60,
	"WBTC":   55,
	"default": 0,
}

// Well-known token addresses on Solana
var knownTokens = map[string]string{
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": "USDC",
	"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": "USDT",
	"So11111111111111111111111111111111111111112":  "SOL",
	"So11111111111111111111111111111111111111111":  "WSOL",
}

// CanonicalPair represents a normalized trading pair
type CanonicalPair struct {
	// BaseToken is the first token (lower priority)
	BaseToken string
	BaseMint  string

	// QuoteToken is the second token (higher priority)
	QuoteToken string
	QuoteMint  string

	// Inverted indicates if we had to swap the original order
	Inverted bool
}

// ResolvePair determines the canonical ordering for a token pair
// Following convention: base/quote where quote is the more "standard" token
// Examples: SOL/USDC, ETH/USDC, BTC/SOL
func ResolvePair(mintA, mintB string) (*CanonicalPair, error) {
	if mintA == "" || mintB == "" {
		return nil, fmt.Errorf("mint addresses cannot be empty")
	}

	// Resolve token symbols
	symbolA := resolveTokenSymbol(mintA)
	symbolB := resolveTokenSymbol(mintB)

	// Get priorities
	priorityA := getQuotePriority(symbolA)
	priorityB := getQuotePriority(symbolB)

	pair := &CanonicalPair{}

	// Higher priority token becomes the quote (second in pair)
	if priorityB > priorityA {
		// B has higher priority, so order is A/B (base/quote)
		pair.BaseToken = symbolA
		pair.BaseMint = mintA
		pair.QuoteToken = symbolB
		pair.QuoteMint = mintB
		pair.Inverted = false
	} else if priorityA > priorityB {
		// A has higher priority, so order is B/A (base/quote)
		pair.BaseToken = symbolB
		pair.BaseMint = mintB
		pair.QuoteToken = symbolA
		pair.QuoteMint = mintA
		pair.Inverted = true
	} else {
		// Equal priority, use lexicographic ordering of mints for consistency
		if mintA < mintB {
			pair.BaseToken = symbolA
			pair.BaseMint = mintA
			pair.QuoteToken = symbolB
			pair.QuoteMint = mintB
			pair.Inverted = false
		} else {
			pair.BaseToken = symbolB
			pair.BaseMint = mintB
			pair.QuoteToken = symbolA
			pair.QuoteMint = mintA
			pair.Inverted = true
		}
	}

	return pair, nil
}

// resolveTokenSymbol returns the symbol for a known token mint, or the mint address if unknown
func resolveTokenSymbol(mint string) string {
	if symbol, ok := knownTokens[mint]; ok {
		return symbol
	}
	// For unknown tokens, return the first 8 characters of the mint as identifier
	if len(mint) > 8 {
		return mint[:8]
	}
	return mint
}

// getQuotePriority returns the priority value for a token symbol
func getQuotePriority(symbol string) int {
	if priority, ok := quotePriority[symbol]; ok {
		return priority
	}
	return quotePriority["default"]
}

// Symbol returns the canonical pair symbol (e.g., "SOL/USDC")
func (p *CanonicalPair) Symbol() string {
	return fmt.Sprintf("%s/%s", p.BaseToken, p.QuoteToken)
}

// IsStablecoinPair returns true if both tokens are stablecoins
func (p *CanonicalPair) IsStablecoinPair() bool {
	stablecoins := map[string]bool{
		"USDC": true,
		"USDT": true,
		"BUSD": true,
		"DAI":  true,
		"UST":  true,
		"FRAX": true,
	}
	return stablecoins[p.BaseToken] && stablecoins[p.QuoteToken]
}

// RegisterToken adds or updates a token in the known tokens registry
// This allows dynamic token registration at runtime
func RegisterToken(mint, symbol string) {
	knownTokens[mint] = symbol
}

// SetQuotePriority sets the priority for a token symbol
// Higher values indicate higher priority as a quote token
func SetQuotePriority(symbol string, priority int) {
	quotePriority[symbol] = priority
}
