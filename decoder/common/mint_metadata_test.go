package common

import (
	"testing"
)

func TestInMemoryMintMetadataProvider_GetMintMetadata(t *testing.T) {
	provider := NewInMemoryMintMetadataProvider()

	tests := []struct {
		name             string
		mintAddress      string
		expectedSymbol   string
		expectedDecimals uint8
		expectError      bool
	}{
		{
			name:             "SOL",
			mintAddress:      "So11111111111111111111111111111111111111112",
			expectedSymbol:   "SOL",
			expectedDecimals: 9,
			expectError:      false,
		},
		{
			name:             "USDC",
			mintAddress:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			expectedSymbol:   "USDC",
			expectedDecimals: 6,
			expectError:      false,
		},
		{
			name:             "USDT",
			mintAddress:      "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			expectedSymbol:   "USDT",
			expectedDecimals: 6,
			expectError:      false,
		},
		{
			name:             "ORCA",
			mintAddress:      "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs",
			expectedSymbol:   "ORCA",
			expectedDecimals: 6,
			expectError:      false,
		},
		{
			name:        "unknown_mint",
			mintAddress: "UnknownMint111111111111111111111111111111",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := provider.GetMintMetadata(tt.mintAddress)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for unknown mint, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMintMetadata failed: %v", err)
			}

			if metadata.Symbol != tt.expectedSymbol {
				t.Errorf("Symbol mismatch: got %s, want %s", metadata.Symbol, tt.expectedSymbol)
			}

			if metadata.Decimals != tt.expectedDecimals {
				t.Errorf("Decimals mismatch: got %d, want %d", metadata.Decimals, tt.expectedDecimals)
			}
		})
	}
}

func TestInMemoryMintMetadataProvider_GetDecimals(t *testing.T) {
	provider := NewInMemoryMintMetadataProvider()

	tests := []struct {
		name             string
		mintAddress      string
		expectedDecimals uint8
		expectError      bool
	}{
		{
			name:             "SOL",
			mintAddress:      "So11111111111111111111111111111111111111112",
			expectedDecimals: 9,
			expectError:      false,
		},
		{
			name:             "USDC",
			mintAddress:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			expectedDecimals: 6,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decimals, err := provider.GetDecimals(tt.mintAddress)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetDecimals failed: %v", err)
			}

			if decimals != tt.expectedDecimals {
				t.Errorf("Decimals mismatch: got %d, want %d", decimals, tt.expectedDecimals)
			}
		})
	}
}

func TestDetermineBaseQuote(t *testing.T) {
	provider := NewInMemoryMintMetadataProvider()

	tests := []struct {
		name          string
		mintA         string
		mintB         string
		expectedBase  string
		expectedQuote string
	}{
		{
			name:          "USDC_SOL",
			mintA:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			mintB:         "So11111111111111111111111111111111111111112",  // SOL
			expectedBase:  "So11111111111111111111111111111111111111112",
			expectedQuote: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		{
			name:          "SOL_USDC_reversed",
			mintA:         "So11111111111111111111111111111111111111112",  // SOL
			mintB:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			expectedBase:  "So11111111111111111111111111111111111111112",
			expectedQuote: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		{
			name:          "USDC_USDT",
			mintA:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			mintB:         "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", // USDT
			expectedBase:  "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			expectedQuote: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		{
			name:          "USDT_SOL",
			mintA:         "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", // USDT
			mintB:         "So11111111111111111111111111111111111111112",  // SOL
			expectedBase:  "So11111111111111111111111111111111111111112",
			expectedQuote: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
		},
		{
			name:          "ORCA_SOL",
			mintA:         "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs", // ORCA
			mintB:         "So11111111111111111111111111111111111111112",  // SOL
			expectedBase:  "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs",
			expectedQuote: "So11111111111111111111111111111111111111112",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, quote, err := DetermineBaseQuote(tt.mintA, tt.mintB, provider)
			if err != nil {
				t.Fatalf("DetermineBaseQuote failed: %v", err)
			}

			if base != tt.expectedBase {
				t.Errorf("Base mismatch: got %s, want %s", base, tt.expectedBase)
			}

			if quote != tt.expectedQuote {
				t.Errorf("Quote mismatch: got %s, want %s", quote, tt.expectedQuote)
			}
		})
	}
}

func TestPriorityMap(t *testing.T) {
	cases := map[string]int{
		"USDC":    100,
		"USDT":    90,
		"SOL":     80,
		"UNKNOWN": 0,
	}

	for symbol, expected := range cases {
		if got := getPriority(symbol); got != expected {
			t.Errorf("priority for %s: got %d, want %d", symbol, got, expected)
		}
	}
}

func TestAddMintMetadata(t *testing.T) {
	provider := NewInMemoryMintMetadataProvider()

	// Add a custom mint
	customMint := &MintMetadata{
		Address:  "CustomMint111111111111111111111111111111111",
		Symbol:   "CUSTOM",
		Decimals: 8,
		Name:     "Custom Token",
	}

	provider.AddMintMetadata(customMint)

	// Verify it was added
	metadata, err := provider.GetMintMetadata(customMint.Address)
	if err != nil {
		t.Fatalf("Failed to get custom mint metadata: %v", err)
	}

	if metadata.Symbol != customMint.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", metadata.Symbol, customMint.Symbol)
	}

	if metadata.Decimals != customMint.Decimals {
		t.Errorf("Decimals mismatch: got %d, want %d", metadata.Decimals, customMint.Decimals)
	}
}
