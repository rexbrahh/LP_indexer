package common

import (
	"testing"
)

func TestResolvePair(t *testing.T) {
	tests := []struct {
		name         string
		mintA        string
		mintB        string
		wantBase     string
		wantQuote    string
		wantInverted bool
		wantErr      bool
	}{
		{
			name:         "SOL/USDC pair",
			mintA:        "So11111111111111111111111111111111111111112",
			mintB:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			wantBase:     "SOL",
			wantQuote:    "USDC",
			wantInverted: false,
			wantErr:      false,
		},
		{
			name:         "USDC/SOL pair (inverted input)",
			mintA:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			mintB:        "So11111111111111111111111111111111111111112",
			wantBase:     "SOL",
			wantQuote:    "USDC",
			wantInverted: true,
			wantErr:      false,
		},
		{
			name:         "SOL/USDT pair",
			mintA:        "So11111111111111111111111111111111111111112",
			mintB:        "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			wantBase:     "SOL",
			wantQuote:    "USDT",
			wantInverted: false,
			wantErr:      false,
		},
		{
			name:         "USDC/USDT pair",
			mintA:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			mintB:        "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			wantBase:     "USDT",
			wantQuote:    "USDC",
			wantInverted: true,
			wantErr:      false,
		},
		{
			name:    "empty mint A",
			mintA:   "",
			mintB:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			wantErr: true,
		},
		{
			name:    "empty mint B",
			mintA:   "So11111111111111111111111111111111111111112",
			mintB:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, err := ResolvePair(tt.mintA, tt.mintB)

			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if pair.BaseToken != tt.wantBase {
				t.Errorf("BaseToken = %v, want %v", pair.BaseToken, tt.wantBase)
			}
			if pair.QuoteToken != tt.wantQuote {
				t.Errorf("QuoteToken = %v, want %v", pair.QuoteToken, tt.wantQuote)
			}
			if pair.Inverted != tt.wantInverted {
				t.Errorf("Inverted = %v, want %v", pair.Inverted, tt.wantInverted)
			}
		})
	}
}

func TestCanonicalPairSymbol(t *testing.T) {
	tests := []struct {
		name       string
		mintA      string
		mintB      string
		wantSymbol string
	}{
		{
			name:       "SOL/USDC",
			mintA:      "So11111111111111111111111111111111111111112",
			mintB:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			wantSymbol: "SOL/USDC",
		},
		{
			name:       "USDC/SOL inverted to SOL/USDC",
			mintA:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			mintB:      "So11111111111111111111111111111111111111112",
			wantSymbol: "SOL/USDC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, err := ResolvePair(tt.mintA, tt.mintB)
			if err != nil {
				t.Fatalf("ResolvePair() error = %v", err)
			}

			symbol := pair.Symbol()
			if symbol != tt.wantSymbol {
				t.Errorf("Symbol() = %v, want %v", symbol, tt.wantSymbol)
			}
		})
	}
}

func TestIsStablecoinPair(t *testing.T) {
	tests := []struct {
		name  string
		mintA string
		mintB string
		want  bool
	}{
		{
			name:  "USDC/USDT is stablecoin pair",
			mintA: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			mintB: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			want:  true,
		},
		{
			name:  "SOL/USDC is not stablecoin pair",
			mintA: "So11111111111111111111111111111111111111112",
			mintB: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, err := ResolvePair(tt.mintA, tt.mintB)
			if err != nil {
				t.Fatalf("ResolvePair() error = %v", err)
			}

			got := pair.IsStablecoinPair()
			if got != tt.want {
				t.Errorf("IsStablecoinPair() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegisterToken(t *testing.T) {
	testMint := "TestMint1111111111111111111111111111111111111"
	testSymbol := "TEST"

	// Register new token
	RegisterToken(testMint, testSymbol)

	// Verify it was registered
	symbol := resolveTokenSymbol(testMint)
	if symbol != testSymbol {
		t.Errorf("resolveTokenSymbol() = %v, want %v", symbol, testSymbol)
	}

	// Clean up
	delete(knownTokens, testMint)
}

func TestSetQuotePriority(t *testing.T) {
	testSymbol := "TESTTOKEN"
	testPriority := 95

	// Set priority
	SetQuotePriority(testSymbol, testPriority)

	// Verify it was set
	priority := getQuotePriority(testSymbol)
	if priority != testPriority {
		t.Errorf("getQuotePriority() = %v, want %v", priority, testPriority)
	}

	// Clean up
	delete(quotePriority, testSymbol)
}

// Benchmark for pair resolution
func BenchmarkResolvePair(b *testing.B) {
	mintA := "So11111111111111111111111111111111111111112"
	mintB := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolvePair(mintA, mintB)
	}
}
