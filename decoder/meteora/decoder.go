package meteora

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/rexbrahh/lp-indexer/decoder/common"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// ErrNotImplemented is retained for compatibility but no longer returned by the
// decoder. Callers may still rely on it when guarding older behaviour.
var ErrNotImplemented = errors.New("meteora decoder not yet implemented")

// ErrUnsupportedInstruction is returned when the provided instruction does not
// match a recognised Meteora swap layout.
var ErrUnsupportedInstruction = errors.New("unsupported meteora swap instruction")

// DecodeSwapEvent accepts raw instruction data and its contextual metadata and
// returns a normalised SwapEvent.
func DecodeSwapEvent(_ []byte, ctx *InstructionContext) (*SwapEvent, error) {
	if ctx == nil {
		return nil, errors.New("instruction context is required")
	}

	if len(ctx.InstructionAccounts) < 8 {
		return nil, fmt.Errorf("%w: insufficient accounts (%d)", ErrUnsupportedInstruction, len(ctx.InstructionAccounts))
	}
	if len(ctx.Accounts) == 0 {
		return nil, fmt.Errorf("%w: empty account key list", ErrUnsupportedInstruction)
	}

	resolveAccount := func(pos int) (string, error) {
		if pos >= len(ctx.InstructionAccounts) {
			return "", fmt.Errorf("instruction missing account at position %d", pos)
		}
		index := uint32(ctx.InstructionAccounts[pos])
		if int(index) >= len(ctx.Accounts) {
			return "", fmt.Errorf("account index %d out of range (len=%d)", index, len(ctx.Accounts))
		}
		return ctx.Accounts[index], nil
	}

	pool, err := resolveAccount(1) // Account order: [pool_authority, pool, ...]
	if err != nil {
		return nil, err
	}

	inputAccountIndex := uint32(ctx.InstructionAccounts[2])
	outputAccountIndex := uint32(ctx.InstructionAccounts[3])

	inputBalance, err := balanceForAccount(ctx, inputAccountIndex)
	if err != nil {
		return nil, fmt.Errorf("input account balance lookup: %w", err)
	}
	outputBalance, err := balanceForAccount(ctx, outputAccountIndex)
	if err != nil {
		return nil, fmt.Errorf("output account balance lookup: %w", err)
	}

	amountIn := inputBalance.outgoing()
	amountOut := outputBalance.incoming()
	if amountIn == 0 && amountOut == 0 {
		return nil, fmt.Errorf("%w: no token balance delta detected", ErrUnsupportedInstruction)
	}

	pair, err := common.ResolvePair(inputBalance.mint, outputBalance.mint)
	if err != nil {
		return nil, fmt.Errorf("resolve canonical pair: %w", err)
	}

	baseMint := pair.BaseMint
	quoteMint := pair.QuoteMint

	baseDecimals := decimalsForMint(ctx, baseMint, inputBalance, outputBalance)
	quoteDecimals := decimalsForMint(ctx, quoteMint, inputBalance, outputBalance)

	var (
		baseAmount    uint64
		quoteAmount   uint64
		baseDecreased bool
	)

	switch {
	case baseMint == inputBalance.mint && quoteMint == outputBalance.mint:
		baseAmount = amountIn
		quoteAmount = amountOut
		baseDecreased = false
	case baseMint == outputBalance.mint && quoteMint == inputBalance.mint:
		baseAmount = amountOut
		quoteAmount = amountIn
		baseDecreased = true
	default:
		// Fall back to the raw deltas if the canonical pair does not match the
		// expected orientation. This should be rare but avoids dropping swaps.
		baseAmount = amountOut
		quoteAmount = amountIn
		baseDecreased = true
	}

	if baseAmount == 0 || quoteAmount == 0 {
		return nil, fmt.Errorf("%w: zero-sized swap (base=%d quote=%d)", ErrUnsupportedInstruction, baseAmount, quoteAmount)
	}

	timestamp := ctx.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Unix(0, 0).UTC()
	}

	event := &SwapEvent{
		Signature: ctx.Signature,
		Slot:      ctx.Slot,
		Timestamp: timestamp,

		ProgramID: ctx.ProgramID,
		Pool:      pool,
		Kind:      ctx.Kind,

		BaseMint:    baseMint,
		QuoteMint:   quoteMint,
		BaseDec:     baseDecimals,
		QuoteDec:    quoteDecimals,
		BaseAmount:  baseAmount,
		QuoteAmount: quoteAmount,

		MintBase:  baseMint,
		MintQuote: quoteMint,
		DecBase:   uint32(baseDecimals),
		DecQuote:  uint32(quoteDecimals),

		BaseDecreased: baseDecreased,
	}

	return event, nil
}

type tokenBalanceDelta struct {
	accountIndex uint32
	mint         string
	decimals     uint8
	pre          uint64
	post         uint64
}

func (b *tokenBalanceDelta) outgoing() uint64 {
	if b == nil {
		return 0
	}
	if b.pre >= b.post {
		return b.pre - b.post
	}
	return 0
}

func (b *tokenBalanceDelta) incoming() uint64 {
	if b == nil {
		return 0
	}
	if b.post >= b.pre {
		return b.post - b.pre
	}
	return 0
}

func balanceForAccount(ctx *InstructionContext, accountIdx uint32) (*tokenBalanceDelta, error) {
	pre := findTokenBalance(ctx.PreTokenBalances, accountIdx)
	post := findTokenBalance(ctx.PostTokenBalances, accountIdx)

	if pre == nil && post == nil {
		return nil, fmt.Errorf("token account %d not present in balance snapshots", accountIdx)
	}

	preAmount, preDecimals, preMint, err := extractBalance(pre)
	if err != nil {
		return nil, fmt.Errorf("pre balance parse: %w", err)
	}
	postAmount, postDecimals, postMint, err := extractBalance(post)
	if err != nil {
		return nil, fmt.Errorf("post balance parse: %w", err)
	}

	mint := pickNonEmpty(preMint, postMint)
	decimals := pickDecimals(preDecimals, postDecimals)

	return &tokenBalanceDelta{
		accountIndex: accountIdx,
		mint:         mint,
		decimals:     decimals,
		pre:          preAmount,
		post:         postAmount,
	}, nil
}

func findTokenBalance(balances []*pb.TokenBalance, accountIdx uint32) *pb.TokenBalance {
	for _, bal := range balances {
		if bal.GetAccountIndex() == accountIdx {
			return bal
		}
	}
	return nil
}

func extractBalance(balance *pb.TokenBalance) (amount uint64, decimals uint8, mint string, err error) {
	if balance == nil {
		return 0, 0, "", nil
	}
	amountStr := balance.GetUiTokenAmount().GetAmount()
	if amountStr == "" {
		amountStr = "0"
	}
	parsed, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("parse amount %q: %w", amountStr, err)
	}

	dec := balance.GetUiTokenAmount().GetDecimals()
	if dec > 255 {
		return 0, 0, "", fmt.Errorf("decimals overflow: %d", dec)
	}
	return parsed, uint8(dec), balance.GetMint(), nil
}

func pickNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func pickDecimals(values ...uint8) uint8 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func decimalsForMint(ctx *InstructionContext, mint string, deltas ...*tokenBalanceDelta) uint8 {
	for _, delta := range deltas {
		if delta != nil && delta.mint == mint {
			return delta.decimals
		}
	}
	for _, bal := range ctx.PreTokenBalances {
		if bal.GetMint() == mint {
			if amt := bal.GetUiTokenAmount(); amt != nil {
				dec := amt.GetDecimals()
				if dec > 255 {
					return 0
				}
				return uint8(dec)
			}
		}
	}
	for _, bal := range ctx.PostTokenBalances {
		if bal.GetMint() == mint {
			if amt := bal.GetUiTokenAmount(); amt != nil {
				dec := amt.GetDecimals()
				if dec > 255 {
					return 0
				}
				return uint8(dec)
			}
		}
	}
	return 0
}
