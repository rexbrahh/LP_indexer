package orca_whirlpool

import (
	"time"

	"github.com/rexbrahh/lp-indexer/.conductor/almaty/decoder/common"
)

// TestFixture represents a test case for decoder validation
type TestFixture struct {
	Name             string
	Signature        string
	Slot             uint64
	Timestamp        time.Time
	InstructionData  []byte
	Accounts         []string
	PreBalances      []uint64
	PostBalances     []uint64
	PoolStatePre     *WhirlpoolState
	PoolStatePost    *WhirlpoolState
	ExpectedEvent    *SwapEvent
}

// GetTestFixtures returns a set of canonical test fixtures
func GetTestFixtures() []TestFixture {
	baseTimestamp := time.Date(2025, 10, 16, 0, 0, 0, 0, time.UTC)

	return []TestFixture{
		// Fixture 1: SOL/USDC swap (SOL -> USDC) - USDC is canonical base
		{
			Name:      "SOL_to_USDC_swap",
			Signature: "5xK5jJ9X...",
			Slot:      250000000,
			Timestamp: baseTimestamp,
			InstructionData: buildSwapInstructionData(
				1000000000,  // 1 SOL (9 decimals)
				0,           // no threshold
				"0",         // no price limit
				true,        // amount is input
				true,        // A to B (SOL to USDC)
			),
			Accounts: []string{
				"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", // token program
				"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM", // authority
				"HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ", // whirlpool (SOL/USDC)
				"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM", // user SOL account
				"5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1", // vault SOL
				"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL", // user USDC account
				"HLmqeL62xR1QoZ1HKKbXRrdN1p3phKpxRMb2VVopvBBz", // vault USDC
			},
			PreBalances: []uint64{
				0, 0, 0,
				5000000000,  // user has 5 SOL
				100000000000, // vault has 100 SOL
				10000000,    // user has 10 USDC
				50000000000, // vault has 50k USDC
			},
			PostBalances: []uint64{
				0, 0, 0,
				4000000000,  // user now has 4 SOL (spent 1)
				101000000000, // vault gained 1 SOL
				10180000000,  // user gained ~180 USDC (@ ~180 USDC/SOL)
				49820000000, // vault lost ~180 USDC
			},
			PoolStatePre: &WhirlpoolState{
				WhirlpoolAddress: "HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ",
				TokenMintA:       "So11111111111111111111111111111111111111112", // SOL
				TokenMintB:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
				TokenVaultA:      "5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1",
				TokenVaultB:      "HLmqeL62xR1QoZ1HKKbXRrdN1p3phKpxRMb2VVopvBBz",
				SqrtPrice:        common.FloatToSqrtPriceQ64(180.0), // 180 USDC per SOL
				TickCurrentIndex: 53215,
				Liquidity:        "5000000000000",
				FeeRate:          30,  // 0.3%
				ProtocolFeeRate:  200, // 2% of fee
			},
			PoolStatePost: &WhirlpoolState{
				WhirlpoolAddress: "HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ",
				TokenMintA:       "So11111111111111111111111111111111111111112",
				TokenMintB:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				TokenVaultA:      "5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1",
				TokenVaultB:      "HLmqeL62xR1QoZ1HKKbXRrdN1p3phKpxRMb2VVopvBBz",
				SqrtPrice:        common.FloatToSqrtPriceQ64(179.5), // slight price movement
				TickCurrentIndex: 53210,
				Liquidity:        "5000000000000",
				FeeRate:          30,
				ProtocolFeeRate:  200,
			},
			ExpectedEvent: &SwapEvent{
				Signature:        "5xK5jJ9X...",
				Slot:             250000000,
				Timestamp:        baseTimestamp,
				PoolAddress:      "HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ",
				MintA:            "So11111111111111111111111111111111111111112",
				MintB:            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AToB:             true,
				AmountIn:         1000000000,
				AmountOut:        10170000000,
				SqrtPriceQ64Pre:  common.FloatToSqrtPriceQ64(180.0),
				SqrtPriceQ64Post: common.FloatToSqrtPriceQ64(179.5),
				TickIndexPre:     53215,
				TickIndexPost:    53210,
				LiquidityPre:     "5000000000000",
				LiquidityPost:    "5000000000000",
				FeeAmount:        3000000, // 0.3% of 1 SOL
				ProtocolFee:      60000,   // 2% of fee
				Price:            179.5,
				VolumeBase:       10170.0, // USDC is base (canonical)
				VolumeQuote:      1.0,     // SOL is quote
				BaseAsset:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
				QuoteAsset:       "So11111111111111111111111111111111111111112",  // SOL
			},
		},

		// Fixture 2: USDC/USDT swap (USDC -> USDT) - USDC is canonical base
		{
			Name:      "USDC_to_USDT_swap",
			Signature: "3mN4kL2...",
			Slot:      250000100,
			Timestamp: baseTimestamp.Add(time.Minute),
			InstructionData: buildSwapInstructionData(
				1000000000, // 1000 USDC (6 decimals)
				0,
				"0",
				true,
				true, // A to B
			),
			Accounts: []string{
				"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
				"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
				"4fuUiYxTQ6QCrdSq9ouBYcTM7bqSwYTSyLueGZLTy4T4", // USDC/USDT pool
				"user_usdc_account",
				"vault_usdc",
				"user_usdt_account",
				"vault_usdt",
			},
			PreBalances: []uint64{
				0, 0, 0,
				5000000000, // 5k USDC
				100000000000,
				3000000000, // 3k USDT
				80000000000,
			},
			PostBalances: []uint64{
				0, 0, 0,
				4000000000,  // spent 1k USDC
				101000000000,
				3999000000,  // gained ~999 USDT (0.1% fee + slippage)
				79001000000,
			},
			PoolStatePre: &WhirlpoolState{
				WhirlpoolAddress: "4fuUiYxTQ6QCrdSq9ouBYcTM7bqSwYTSyLueGZLTy4T4",
				TokenMintA:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
				TokenMintB:       "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", // USDT
				TokenVaultA:      "vault_usdc",
				TokenVaultB:      "vault_usdt",
				SqrtPrice:        common.FloatToSqrtPriceQ64(1.001),
				TickCurrentIndex: 10,
				Liquidity:        "10000000000000",
				FeeRate:          10, // 0.1% for stablecoin pairs
				ProtocolFeeRate:  100,
			},
			PoolStatePost: &WhirlpoolState{
				WhirlpoolAddress: "4fuUiYxTQ6QCrdSq9ouBYcTM7bqSwYTSyLueGZLTy4T4",
				TokenMintA:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				TokenMintB:       "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
				TokenVaultA:      "vault_usdc",
				TokenVaultB:      "vault_usdt",
				SqrtPrice:        common.FloatToSqrtPriceQ64(1.0009),
				TickCurrentIndex: 9,
				Liquidity:        "10000000000000",
				FeeRate:          10,
				ProtocolFeeRate:  100,
			},
			ExpectedEvent: &SwapEvent{
				Signature:        "3mN4kL2...",
				Slot:             250000100,
				Timestamp:        baseTimestamp.Add(time.Minute),
				PoolAddress:      "4fuUiYxTQ6QCrdSq9ouBYcTM7bqSwYTSyLueGZLTy4T4",
				MintA:            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				MintB:            "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
				AToB:             true,
				AmountIn:         1000000000,
				AmountOut:        999000000,
				SqrtPriceQ64Pre:  common.FloatToSqrtPriceQ64(1.001),
				SqrtPriceQ64Post: common.FloatToSqrtPriceQ64(1.0009),
				TickIndexPre:     10,
				TickIndexPost:    9,
				LiquidityPre:     "10000000000000",
				LiquidityPost:    "10000000000000",
				FeeAmount:        1000000, // 0.1%
				ProtocolFee:      10000,
				Price:            1.0009,
				VolumeBase:       1000.0, // USDC is base
				VolumeQuote:      999.0,  // USDT is quote
				BaseAsset:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				QuoteAsset:       "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			},
		},
	}
}

// buildSwapInstructionData constructs raw instruction data for testing
func buildSwapInstructionData(
	amount uint64,
	otherAmountThreshold uint64,
	sqrtPriceLimit string,
	amountSpecifiedIsInput bool,
	aToB bool,
) []byte {
	data := make([]byte, 0, 49)

	// Discriminator (8 bytes) - 0xf8c69e91e17587c8
	discriminatorBytes := make([]byte, 8)
	discriminatorBytes[0] = 0xc8
	discriminatorBytes[1] = 0x87
	discriminatorBytes[2] = 0x75
	discriminatorBytes[3] = 0xe1
	discriminatorBytes[4] = 0x91
	discriminatorBytes[5] = 0x9e
	discriminatorBytes[6] = 0xc6
	discriminatorBytes[7] = 0xf8
	data = append(data, discriminatorBytes...)

	// Amount (8 bytes)
	amountBytes := make([]byte, 8)
	amountBytes[0] = byte(amount)
	amountBytes[1] = byte(amount >> 8)
	amountBytes[2] = byte(amount >> 16)
	amountBytes[3] = byte(amount >> 24)
	amountBytes[4] = byte(amount >> 32)
	amountBytes[5] = byte(amount >> 40)
	amountBytes[6] = byte(amount >> 48)
	amountBytes[7] = byte(amount >> 56)
	data = append(data, amountBytes...)

	// OtherAmountThreshold (8 bytes)
	thresholdBytes := make([]byte, 8)
	thresholdBytes[0] = byte(otherAmountThreshold)
	thresholdBytes[1] = byte(otherAmountThreshold >> 8)
	thresholdBytes[2] = byte(otherAmountThreshold >> 16)
	thresholdBytes[3] = byte(otherAmountThreshold >> 24)
	thresholdBytes[4] = byte(otherAmountThreshold >> 32)
	thresholdBytes[5] = byte(otherAmountThreshold >> 40)
	thresholdBytes[6] = byte(otherAmountThreshold >> 48)
	thresholdBytes[7] = byte(otherAmountThreshold >> 56)
	data = append(data, thresholdBytes...)

	// SqrtPriceLimit (16 bytes for u128)
	sqrtPriceLimitBytes := make([]byte, 16)
	data = append(data, sqrtPriceLimitBytes...)

	// AmountSpecifiedIsInput (1 byte)
	if amountSpecifiedIsInput {
		data = append(data, 1)
	} else {
		data = append(data, 0)
	}

	// AToB (1 byte)
	if aToB {
		data = append(data, 1)
	} else {
		data = append(data, 0)
	}

	return data
}
