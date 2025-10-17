package geyser

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mr-tron/base58/base58"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
	"github.com/rexbrahh/lp-indexer/ingestor/common"
	"github.com/rexbrahh/lp-indexer/ingestor/geyser/internal"
	"github.com/rexbrahh/lp-indexer/observability"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

const chainIDSolana = 501

// SwapPublisher publishes canonical swap events.
type SwapPublisher interface {
	PublishSwap(ctx context.Context, event *dexv1.SwapEvent) error
}

// Processor consumes geyser transaction updates and emits Raydium swaps.
type Processor struct {
	publisher  SwapPublisher
	slotCache  common.SlotTimeCache
	metrics    *processorMetrics
	poolConfig map[string]string
	poolFees   map[string]uint16
	configFees map[string]uint16
}

// NewProcessor initialises a Processor with optional metrics registration.
func NewProcessor(publisher SwapPublisher, cache common.SlotTimeCache, reg prometheus.Registerer) *Processor {
	if cache == nil {
		cache = common.NewMemorySlotTimeCache()
	}
	return &Processor{
		publisher:  publisher,
		slotCache:  cache,
		metrics:    newProcessorMetrics(reg),
		poolConfig: make(map[string]string),
		poolFees:   make(map[string]uint16),
		configFees: make(map[string]uint16),
	}
}

// HandleUpdate inspects the geyser update and decodes Raydium swaps when
// present.
func (p *Processor) HandleUpdate(ctx context.Context, update *pb.SubscribeUpdate) error {
	switch u := update.GetUpdateOneof().(type) {
	case *pb.SubscribeUpdate_Transaction:
		return p.handleTransaction(ctx, u.Transaction)
	case *pb.SubscribeUpdate_BlockMeta:
		p.handleBlockMeta(u.BlockMeta)
		return nil
	case *pb.SubscribeUpdate_Account:
		p.handleAccount(u.Account)
		return nil
	default:
		return nil
	}
}

func (p *Processor) handleTransaction(ctx context.Context, tx *pb.SubscribeUpdateTransaction) error {
	if tx == nil {
		return nil
	}
	events, err := p.decodeRaydiumSwaps(tx)
	if err != nil {
		p.metrics.errors.Inc()
		return err
	}
	for _, ev := range events {
		if err := p.publisher.PublishSwap(ctx, ev); err != nil {
			p.metrics.errors.Inc()
			return fmt.Errorf("publish swap: %w", err)
		}
		p.metrics.swaps.Inc()
	}
	return nil
}

func (p *Processor) handleBlockMeta(meta *pb.SubscribeUpdateBlockMeta) {
	if meta == nil {
		return
	}
	if ts := meta.GetBlockTime(); ts != nil {
		timeValue := time.Unix(ts.GetTimestamp(), 0).UTC()
		p.slotCache.Set(meta.GetSlot(), timeValue)
	}
}

func (p *Processor) handleAccount(account *pb.SubscribeUpdateAccount) {
	if account == nil || account.Account == nil {
		return
	}
	info := account.Account
	owner := base58.Encode(info.GetOwner())
	if owner != ray.ProgramID {
		return
	}
	pubkey := base58.Encode(info.GetPubkey())
	data := info.GetData()

	if internal.HasPoolDiscriminator(data) {
		if cfg, err := internal.DecodeRaydiumPool(data); err == nil {
			configKey := base58.Encode(cfg)
			p.poolConfig[pubkey] = configKey
			if fee, ok := p.configFees[configKey]; ok {
				p.poolFees[pubkey] = fee
			}
			return
		}
	}

	if internal.HasAmmConfigDiscriminator(data) {
		if tradeRate, err := internal.DecodeAmmConfig(data); err == nil {
			feeBps := uint16(tradeRate / 100)
			p.configFees[pubkey] = feeBps
			for pool, cfg := range p.poolConfig {
				if cfg == pubkey {
					p.poolFees[pool] = feeBps
				}
			}
			return
		}
	}

	if len(data) > internal.ApproxConfigAccountMax {
		if cfg, err := internal.DecodeRaydiumPool(data); err == nil {
			configKey := base58.Encode(cfg)
			p.poolConfig[pubkey] = configKey
			if fee, ok := p.configFees[configKey]; ok {
				p.poolFees[pubkey] = fee
			}
			return
		}
	}

	if tradeRate, err := internal.DecodeAmmConfig(data); err == nil {
		feeBps := uint16(tradeRate / 100)
		p.configFees[pubkey] = feeBps
		for pool, cfg := range p.poolConfig {
			if cfg == pubkey {
				p.poolFees[pool] = feeBps
			}
		}
		return
	}

	if cfg, err := internal.DecodeRaydiumPool(data); err == nil {
		configKey := base58.Encode(cfg)
		p.poolConfig[pubkey] = configKey
		if fee, ok := p.configFees[configKey]; ok {
			p.poolFees[pubkey] = fee
		}
	}
}

func (p *Processor) decodeRaydiumSwaps(tx *pb.SubscribeUpdateTransaction) ([]*dexv1.SwapEvent, error) {
	info := tx.GetTransaction()
	if info == nil {
		return nil, nil
	}
	meta := info.GetMeta()
	if meta == nil {
		return nil, nil
	}
	txMsg := info.GetTransaction()
	if txMsg == nil {
		return nil, nil
	}
	message := txMsg.GetMessage()
	if message == nil {
		return nil, nil
	}

	accountStrs := make([]string, len(message.GetAccountKeys()))
	for i, key := range message.GetAccountKeys() {
		accountStrs[i] = base58.Encode(key)
	}

	programIndex := -1
	for idx, key := range accountStrs {
		if key == ray.ProgramID {
			programIndex = idx
			break
		}
	}
	if programIndex < 0 {
		return nil, nil
	}

	vaults := extractVaultBalances(meta)
	if len(vaults) == 0 {
		return nil, nil
	}

	signature := encodeSignature(txMsg.GetSignatures())
	slot := tx.GetSlot()
	timestamp := lookupSlotTimestamp(p.slotCache, slot)
	index := info.GetIndex()

	var swaps []*dexv1.SwapEvent
	for _, instr := range message.GetInstructions() {
		if int(instr.GetProgramIdIndex()) != programIndex {
			continue
		}
		ev, err := p.buildRaydiumSwap(signature, slot, timestamp, index, instr, accountStrs, vaults)
		if err != nil {
			return nil, fmt.Errorf("decode raydium swap: %w", err)
		}
		if ev != nil {
			swaps = append(swaps, ev)
		}
	}
	return swaps, nil
}

func (p *Processor) buildRaydiumSwap(signature string, slot uint64, timestamp int64, index uint64, instr *pb.CompiledInstruction, accountStrs []string, vaults map[string][]*tokenBalance) (*dexv1.SwapEvent, error) {
	data := instr.GetData()
	if len(data) == 0 {
		return nil, nil
	}

	pool, vaultA, vaultB := resolvePool(instr, accountStrs, vaults)
	if pool == "" || vaultA == nil || vaultB == nil {
		return nil, nil
	}

	swapInstr, err := ray.ParseSwapInstruction(data)
	if err != nil {
		return nil, fmt.Errorf("parse swap instruction: %w", err)
	}

	ctx := &ray.SwapContext{
		Accounts: ray.AccountKeys{
			PoolAddress: pool,
			MintA:       vaultA.mint,
			MintB:       vaultB.mint,
		},
		PreTokenA:  vaultA.pre,
		PostTokenA: vaultA.post,
		PreTokenB:  vaultB.pre,
		PostTokenB: vaultB.post,
		DecimalsA:  vaultA.decimals,
		DecimalsB:  vaultB.decimals,
		FeeBps:     0,
		Slot:       slot,
		Signature:  signature,
		Timestamp:  timestamp,
	}

	rayEvent, err := ray.ParseSwapEvent(swapInstr, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse swap event: %w", err)
	}

	feeBps := p.poolFees[pool]
	return convertRaydiumSwap(rayEvent, slot, timestamp, signature, index, feeBps), nil
}

func convertRaydiumSwap(ev *ray.SwapEvent, slot uint64, timestamp int64, signature string, index uint64, feeBps uint16) *dexv1.SwapEvent {
	msg := &dexv1.SwapEvent{
		ChainId:          chainIDSolana,
		Slot:             slot,
		Sig:              signature,
		Index:            uint32(index),
		ProgramId:        ray.ProgramID,
		PoolId:           ev.PoolAddress,
		MintBase:         ev.MintA,
		MintQuote:        ev.MintB,
		DecBase:          uint32(ev.DecimalsA),
		DecQuote:         uint32(ev.DecimalsB),
		SqrtPriceQ64Pre:  ev.SqrtPriceX64Low,
		SqrtPriceQ64Post: ev.SqrtPriceX64High,
		FeeBps:           uint32(feeBps),
		Provisional:      true,
	}

	if ev.IsBaseInput {
		msg.BaseIn = ev.AmountIn
		msg.QuoteOut = ev.AmountOut
	} else {
		msg.BaseOut = ev.AmountOut
		msg.QuoteIn = ev.AmountIn
	}

	return msg
}

type tokenBalance struct {
	accountIndex uint32
	mint         string
	owner        string
	pre          uint64
	post         uint64
	decimals     uint8
}

func extractVaultBalances(meta *pb.TransactionStatusMeta) map[string][]*tokenBalance {
	balances := map[uint32]*tokenBalance{}

	for _, bal := range meta.GetPreTokenBalances() {
		amt, err := parseAmount(bal.GetUiTokenAmount(), bal.GetMint())
		if err != nil {
			continue
		}
		balances[bal.GetAccountIndex()] = &tokenBalance{
			accountIndex: bal.GetAccountIndex(),
			mint:         bal.GetMint(),
			owner:        bal.GetOwner(),
			pre:          amt,
			decimals:     uint8(bal.GetUiTokenAmount().GetDecimals()),
		}
	}

	for _, bal := range meta.GetPostTokenBalances() {
		amt, err := parseAmount(bal.GetUiTokenAmount(), bal.GetMint())
		if err != nil {
			continue
		}
		entry, ok := balances[bal.GetAccountIndex()]
		if !ok {
			entry = &tokenBalance{
				accountIndex: bal.GetAccountIndex(),
				mint:         bal.GetMint(),
				owner:        bal.GetOwner(),
				decimals:     uint8(bal.GetUiTokenAmount().GetDecimals()),
			}
			balances[bal.GetAccountIndex()] = entry
		}
		entry.post = amt
	}

	owners := map[string][]*tokenBalance{}
	for _, bal := range balances {
		if bal.owner == "" {
			continue
		}
		owners[bal.owner] = append(owners[bal.owner], bal)
	}
	return owners
}

func resolvePool(instr *pb.CompiledInstruction, accountStrs []string, vaults map[string][]*tokenBalance) (string, *tokenBalance, *tokenBalance) {
	var pool string
	var ordered []*tokenBalance

	for _, rawIdx := range instr.GetAccounts() {
		idx := int(rawIdx)
		if idx >= len(accountStrs) {
			continue
		}
		addr := accountStrs[idx]
		if len(pool) == 0 {
			if _, ok := vaults[addr]; ok {
				pool = addr
			}
		}
	}

	if pool == "" {
		return "", nil, nil
	}

	poolVaults := vaults[pool]
	for _, rawIdx := range instr.GetAccounts() {
		idx := int(rawIdx)
		for _, tb := range poolVaults {
			if tb.accountIndex == uint32(idx) {
				ordered = append(ordered, tb)
			}
		}
	}

	if len(ordered) < 2 {
		ordered = poolVaults
	}
	if len(ordered) < 2 {
		return "", nil, nil
	}
	return pool, ordered[0], ordered[1]
}

func parseAmount(amount *pb.UiTokenAmount, mint string) (uint64, error) {
	if amount == nil {
		return 0, fmt.Errorf("missing ui amount for mint %s", mint)
	}
	val := amount.GetAmount()
	if val == "" {
		return 0, fmt.Errorf("empty amount for mint %s", mint)
	}
	parsed, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %s for mint %s: %w", val, mint, err)
	}
	return parsed, nil
}

func encodeSignature(signatures [][]byte) string {
	if len(signatures) == 0 {
		return ""
	}
	return base58.Encode(signatures[0])
}

func lookupSlotTimestamp(cache common.SlotTimeCache, slot uint64) int64 {
	if cache == nil {
		return 0
	}
	ts, err := cache.Get(slot)
	if err != nil || ts.IsZero() {
		return 0
	}
	return ts.Unix()
}

type processorMetrics struct {
	swaps  prometheus.Counter
	errors prometheus.Counter
}

func newProcessorMetrics(reg prometheus.Registerer) *processorMetrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	return &processorMetrics{
		swaps: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricRaydiumSwapsTotal,
			Help:      "Total Raydium swaps decoded from geyser transactions.",
		}),
		errors: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricRaydiumDecodeErrors,
			Help:      "Raydium swap decode or publish errors.",
		}),
	}
}
