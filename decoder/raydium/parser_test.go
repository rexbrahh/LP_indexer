package raydium

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFixture represents the structure of our test fixture JSON files
type TestFixture struct {
	Description      string `json:"description"`
	Signature        string `json:"signature"`
	Slot             uint64 `json:"slot"`
	Timestamp        int64  `json:"timestamp"`
	PoolAddress      string `json:"pool_address"`
	MintA            string `json:"mint_a"`
	MintB            string `json:"mint_b"`
	DecimalsA        uint8  `json:"decimals_a"`
	DecimalsB        uint8  `json:"decimals_b"`
	FeeBps           uint16 `json:"fee_bps"`
	InstructionData  string `json:"instruction_data"`
	PreVaultA        uint64 `json:"pre_vault_a"`
	PostVaultA       uint64 `json:"post_vault_a"`
	PreVaultB        uint64 `json:"pre_vault_b"`
	PostVaultB       uint64 `json:"post_vault_b"`
	SqrtPriceX64Low  uint64 `json:"sqrt_price_x64_low"`
	SqrtPriceX64High uint64 `json:"sqrt_price_x64_high"`
	ExpectedAmountIn  uint64  `json:"expected_amount_in"`
	ExpectedAmountOut uint64  `json:"expected_amount_out"`
	ExpectedPrice     float64 `json:"expected_price"`
	ExpectedVolume    uint64  `json:"expected_volume"`
	Notes            string  `json:"notes"`
}

func loadTestFixture(t *testing.T, filename string) *TestFixture {
	t.Helper()

	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", filename, err)
	}

	var fixture TestFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("failed to unmarshal fixture %s: %v", filename, err)
	}

	return &fixture
}

func TestParseSwapInstruction(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
	}{
		{
			name:    "SOL to USDC swap",
			fixture: "swap_tx_1.json",
			wantErr: false,
		},
		{
			name:    "USDC to SOL swap",
			fixture: "swap_tx_2.json",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadTestFixture(t, tt.fixture)

			// Decode instruction data
			instrData, err := hex.DecodeString(fixture.InstructionData)
			if err != nil {
				t.Fatalf("failed to decode instruction data: %v", err)
			}

			// Parse instruction
			instr, err := ParseSwapInstruction(instrData)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSwapInstruction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify instruction was parsed
			if instr == nil {
				t.Fatal("expected non-nil instruction")
			}

			// Basic validation that we got a valid instruction
			if instr.Amount == 0 {
				t.Error("instruction amount should not be zero")
			}
		})
	}
}

func TestParseSwapEvent(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{
			name:    "SOL to USDC swap",
			fixture: "swap_tx_1.json",
		},
		{
			name:    "USDC to SOL swap",
			fixture: "swap_tx_2.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadTestFixture(t, tt.fixture)

			// Decode instruction data
			instrData, err := hex.DecodeString(fixture.InstructionData)
			if err != nil {
				t.Fatalf("failed to decode instruction data: %v", err)
			}

			// Parse instruction
			instr, err := ParseSwapInstruction(instrData)
			if err != nil {
				t.Fatalf("ParseSwapInstruction() error = %v", err)
			}

			// Build context
			ctx := &SwapContext{
				Accounts: AccountKeys{
					PoolAddress: fixture.PoolAddress,
					MintA:       fixture.MintA,
					MintB:       fixture.MintB,
				},
				PreTokenA:  fixture.PreVaultA,
				PostTokenA: fixture.PostVaultA,
				PreTokenB:  fixture.PreVaultB,
				PostTokenB: fixture.PostVaultB,
				DecimalsA:  fixture.DecimalsA,
				DecimalsB:  fixture.DecimalsB,
				FeeBps:     fixture.FeeBps,
				Slot:       fixture.Slot,
				Signature:  fixture.Signature,
				Timestamp:  fixture.Timestamp,
			}

			// Parse swap event
			event, err := ParseSwapEvent(instr, ctx)
			if err != nil {
				t.Fatalf("ParseSwapEvent() error = %v", err)
			}

			// Verify canonical fields are populated
			if event.PoolAddress != fixture.PoolAddress {
				t.Errorf("PoolAddress = %v, want %v", event.PoolAddress, fixture.PoolAddress)
			}
			if event.MintA != fixture.MintA {
				t.Errorf("MintA = %v, want %v", event.MintA, fixture.MintA)
			}
			if event.MintB != fixture.MintB {
				t.Errorf("MintB = %v, want %v", event.MintB, fixture.MintB)
			}
			if event.DecimalsA != fixture.DecimalsA {
				t.Errorf("DecimalsA = %v, want %v", event.DecimalsA, fixture.DecimalsA)
			}
			if event.DecimalsB != fixture.DecimalsB {
				t.Errorf("DecimalsB = %v, want %v", event.DecimalsB, fixture.DecimalsB)
			}
			if event.FeeBps != fixture.FeeBps {
				t.Errorf("FeeBps = %v, want %v", event.FeeBps, fixture.FeeBps)
			}

			// Verify amounts match expected values
			if event.AmountIn != fixture.ExpectedAmountIn {
				t.Errorf("AmountIn = %v, want %v", event.AmountIn, fixture.ExpectedAmountIn)
			}
			if event.AmountOut != fixture.ExpectedAmountOut {
				t.Errorf("AmountOut = %v, want %v", event.AmountOut, fixture.ExpectedAmountOut)
			}

			// Verify transaction context
			if event.Slot != fixture.Slot {
				t.Errorf("Slot = %v, want %v", event.Slot, fixture.Slot)
			}
			if event.Signature != fixture.Signature {
				t.Errorf("Signature = %v, want %v", event.Signature, fixture.Signature)
			}
			if event.Timestamp != fixture.Timestamp {
				t.Errorf("Timestamp = %v, want %v", event.Timestamp, fixture.Timestamp)
			}
		})
	}
}

func TestCalculateVolume(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{
			name:    "SOL to USDC swap volume",
			fixture: "swap_tx_1.json",
		},
		{
			name:    "USDC to SOL swap volume",
			fixture: "swap_tx_2.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadTestFixture(t, tt.fixture)

			// Decode and parse instruction
			instrData, err := hex.DecodeString(fixture.InstructionData)
			if err != nil {
				t.Fatalf("failed to decode instruction data: %v", err)
			}

			instr, err := ParseSwapInstruction(instrData)
			if err != nil {
				t.Fatalf("ParseSwapInstruction() error = %v", err)
			}

			// Build context and parse event
			ctx := &SwapContext{
				Accounts: AccountKeys{
					PoolAddress: fixture.PoolAddress,
					MintA:       fixture.MintA,
					MintB:       fixture.MintB,
				},
				PreTokenA:  fixture.PreVaultA,
				PostTokenA: fixture.PostVaultA,
				PreTokenB:  fixture.PreVaultB,
				PostTokenB: fixture.PostVaultB,
				DecimalsA:  fixture.DecimalsA,
				DecimalsB:  fixture.DecimalsB,
				FeeBps:     fixture.FeeBps,
				Slot:       fixture.Slot,
				Signature:  fixture.Signature,
				Timestamp:  fixture.Timestamp,
			}

			event, err := ParseSwapEvent(instr, ctx)
			if err != nil {
				t.Fatalf("ParseSwapEvent() error = %v", err)
			}

			// Calculate volume
			volume := event.CalculateVolume()

			// Verify volume matches expected
			if volume != fixture.ExpectedVolume {
				t.Errorf("CalculateVolume() = %v, want %v", volume, fixture.ExpectedVolume)
			}
		})
	}
}

func TestCalculatePrice(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		epsilon float64 // Tolerance for floating-point comparison
	}{
		{
			name:    "SOL to USDC price",
			fixture: "swap_tx_1.json",
			epsilon: 0.01, // Allow 1% tolerance for Q64.64 conversion
		},
		{
			name:    "USDC to SOL price",
			fixture: "swap_tx_2.json",
			epsilon: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadTestFixture(t, tt.fixture)

			// Decode and parse instruction
			instrData, err := hex.DecodeString(fixture.InstructionData)
			if err != nil {
				t.Fatalf("failed to decode instruction data: %v", err)
			}

			instr, err := ParseSwapInstruction(instrData)
			if err != nil {
				t.Fatalf("ParseSwapInstruction() error = %v", err)
			}

			// Build context and parse event
			ctx := &SwapContext{
				Accounts: AccountKeys{
					PoolAddress: fixture.PoolAddress,
					MintA:       fixture.MintA,
					MintB:       fixture.MintB,
				},
				PreTokenA:  fixture.PreVaultA,
				PostTokenA: fixture.PostVaultA,
				PreTokenB:  fixture.PreVaultB,
				PostTokenB: fixture.PostVaultB,
				DecimalsA:  fixture.DecimalsA,
				DecimalsB:  fixture.DecimalsB,
				FeeBps:     fixture.FeeBps,
				Slot:       fixture.Slot,
				Signature:  fixture.Signature,
				Timestamp:  fixture.Timestamp,
			}

			event, err := ParseSwapEvent(instr, ctx)
			if err != nil {
				t.Fatalf("ParseSwapEvent() error = %v", err)
			}

			// Calculate price
			price := event.CalculatePrice()

			// Verify price is within tolerance
			diff := abs(int(price - fixture.ExpectedPrice))
			tolerance := fixture.ExpectedPrice * tt.epsilon
			if float64(diff) > tolerance {
				t.Errorf("CalculatePrice() = %v, want %v (±%v)", price, fixture.ExpectedPrice, tolerance)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkParseSwapInstruction(b *testing.B) {
	// Sample instruction data
	instrData, _ := hex.DecodeString("f8c69e91e17587c8010000000000000040420f0000000000000000000000000000000000000000000001")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSwapInstruction(instrData)
	}
}

func BenchmarkParseSwapEvent(b *testing.B) {
	instrData, _ := hex.DecodeString("f8c69e91e17587c8010000000000000040420f0000000000000000000000000000000000000000000001")
	instr, _ := ParseSwapInstruction(instrData)

	ctx := &SwapContext{
		Accounts: AccountKeys{
			PoolAddress: "6UmmUiYoBjSrhakAobJw8BvkmJtDVxaeBtbt7rxWo1mg",
			MintA:       "So11111111111111111111111111111111111111112",
			MintB:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		},
		PreTokenA:  15234567890123,
		PostTokenA: 15235567890123,
		PreTokenB:  98765432109876,
		PostTokenB: 98765372109876,
		DecimalsA:  9,
		DecimalsB:  6,
		FeeBps:     25,
		Slot:       245123456,
		Signature:  "5wJwKxPzF3QnhU8vN2K9L7tYmF4xB1cA9qR8pD6sE3mH2jT4vC7nW1kS9pX5rZ8yQ",
		Timestamp:  1704067200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSwapEvent(instr, ctx)
	}
}
