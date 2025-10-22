package decoder

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mr-tron/base58/base58"

	meteora "github.com/rexbrahh/lp-indexer/decoder/meteora"
	orcawhirlpool "github.com/rexbrahh/lp-indexer/decoder/orca_whirlpool"
	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
	"github.com/rexbrahh/lp-indexer/ingestor/common"
	poolmeta "github.com/rexbrahh/lp-indexer/ingestor/internal/pools"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

const chainIDSolana = 501

// Decoder maintains the shared state required to decode swap transactions
// emitted by both Yellowstone Geyser and Helius streams.
type Decoder struct {
	slotCache  common.SlotTimeCache
	poolConfig map[string]string
	poolFees   map[string]uint16
	configFees map[string]uint16
	orcaPools  map[string]*poolmeta.OrcaPoolInfo
}

// New constructs a decoder using the provided slot cache. When cache is nil a
// new in-memory cache is created.
func New(cache common.SlotTimeCache) *Decoder {
	if cache == nil {
		cache = common.NewMemorySlotTimeCache()
	}
	return &Decoder{
		slotCache:  cache,
		poolConfig: make(map[string]string),
		poolFees:   make(map[string]uint16),
		configFees: make(map[string]uint16),
		orcaPools:  make(map[string]*poolmeta.OrcaPoolInfo),
	}
}

// SlotCache exposes the underlying cache so callers can share it with other
// components (e.g. block head consumers).
func (d *Decoder) SlotCache() common.SlotTimeCache {
	return d.slotCache
}

// HandleBlockMeta updates the slot→timestamp cache with block metadata.
func (d *Decoder) HandleBlockMeta(meta *pb.SubscribeUpdateBlockMeta) {
	if meta == nil {
		return
	}
	if ts := meta.GetBlockTime(); ts != nil && d.slotCache != nil {
		d.slotCache.Set(meta.GetSlot(), time.Unix(ts.GetTimestamp(), 0).UTC())
	}
}

// HandleAccount indexes account data used to enrich swap decoding (e.g. pool
// configuration, fee rates, and Orca pool metadata).
func (d *Decoder) HandleAccount(account *pb.SubscribeUpdateAccount) {
	if account == nil || account.Account == nil {
		return
	}
	info := account.Account
	owner := base58.Encode(info.GetOwner())
	pubkey := base58.Encode(info.GetPubkey())
	data := info.GetData()

	switch owner {
	case ray.ProgramID:
		d.handleRaydiumAccount(pubkey, data)
	case orcawhirlpool.WhirlpoolProgramID:
		if poolInfo, err := poolmeta.DecodeOrcaPool(data); err == nil {
			d.orcaPools[pubkey] = poolInfo
		}
	}
}

// DecodeTransaction inspects the provided transaction update and returns any
// decoded swap events (Raydium, Orca Whirlpool, Meteora). When decoding fails
// for a recognised program a *DecodeError is returned.
func (d *Decoder) DecodeTransaction(tx *pb.SubscribeUpdateTransaction) ([]*dexv1.SwapEvent, error) {
	if tx == nil {
		return nil, nil
	}

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

	vaults := extractVaultBalances(meta)
	signature := encodeSignature(txMsg.GetSignatures())
	slot := tx.GetSlot()
	timestamp := lookupSlotTimestamp(d.slotCache, slot)
	index := info.GetIndex()

	var events []*dexv1.SwapEvent

	for _, instr := range message.GetInstructions() {
		programIdx := int(instr.GetProgramIdIndex())
		if programIdx >= len(accountStrs) {
			continue
		}
		programID := accountStrs[programIdx]

		switch programID {
		case ray.ProgramID:
			ev, err := d.buildRaydiumSwap(signature, slot, timestamp, index, instr, accountStrs, vaults)
			if err != nil {
				return nil, &DecodeError{Program: ray.ProgramID, Err: err}
			}
			if ev != nil {
				events = append(events, ev)
			}
		case orcawhirlpool.WhirlpoolProgramID:
			ev, err := d.buildOrcaSwap(signature, slot, timestamp, index, instr, accountStrs, vaults)
			if err != nil {
				return nil, &DecodeError{Program: orcawhirlpool.WhirlpoolProgramID, Err: err}
			}
			if ev != nil {
				events = append(events, ev)
			}
		default:
			if kind, ok := meteora.ProgramKindForID(programID); ok {
				ev, err := d.buildMeteoraSwap(signature, slot, timestamp, index, instr, accountStrs, meta, programID, kind)
				if err != nil {
					return nil, &DecodeError{Program: programID, Err: err}
				}
				if ev != nil {
					events = append(events, ev)
				}
			}
		}
	}

	return events, nil
}

// DecodeError annotates decode failures with the program identifier.
type DecodeError struct {
	Program string
	Err     error
}

func (e *DecodeError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Program, e.Err)
}

func (e *DecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// --- internal helpers ---

func (d *Decoder) handleRaydiumAccount(pubkey string, data []byte) {
	if poolmeta.HasPoolDiscriminator(data) {
		if cfg, err := poolmeta.DecodeRaydiumPool(data); err == nil {
			configKey := base58.Encode(cfg)
			d.poolConfig[pubkey] = configKey
			if fee, ok := d.configFees[configKey]; ok {
				d.poolFees[pubkey] = fee
			}
			return
		}
	}

	if poolmeta.HasAmmConfigDiscriminator(data) {
		if tradeRate, err := poolmeta.DecodeAmmConfig(data); err == nil {
			feeBps := uint16(tradeRate / 100)
			d.configFees[pubkey] = feeBps
			for pool, cfg := range d.poolConfig {
				if cfg == pubkey {
					d.poolFees[pool] = feeBps
				}
			}
			return
		}
	}

	if len(data) > poolmeta.ApproxConfigAccountMax {
		if cfg, err := poolmeta.DecodeRaydiumPool(data); err == nil {
			configKey := base58.Encode(cfg)
			d.poolConfig[pubkey] = configKey
			if fee, ok := d.configFees[configKey]; ok {
				d.poolFees[pubkey] = fee
			}
			return
		}
	}

	if tradeRate, err := poolmeta.DecodeAmmConfig(data); err == nil {
		feeBps := uint16(tradeRate / 100)
		d.configFees[pubkey] = feeBps
		for pool, cfg := range d.poolConfig {
			if cfg == pubkey {
				d.poolFees[pool] = feeBps
			}
		}
		return
	}

	if cfg, err := poolmeta.DecodeRaydiumPool(data); err == nil {
		configKey := base58.Encode(cfg)
		d.poolConfig[pubkey] = configKey
		if fee, ok := d.configFees[configKey]; ok {
			d.poolFees[pubkey] = fee
		}
	}
}

func (d *Decoder) buildRaydiumSwap(signature string, slot uint64, timestamp int64, index uint64, instr *pb.CompiledInstruction, accountStrs []string, vaults map[string][]*tokenBalance) (*dexv1.SwapEvent, error) {
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

	event, err := ray.ParseSwapEvent(swapInstr, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse swap event: %w", err)
	}

	feeBps := d.poolFees[pool]
	return convertRaydiumSwap(event, slot, timestamp, signature, index, feeBps), nil
}

func (d *Decoder) buildOrcaSwap(signature string, slot uint64, timestamp int64, index uint64, instr *pb.CompiledInstruction, accountStrs []string, vaults map[string][]*tokenBalance) (*dexv1.SwapEvent, error) {
	accounts := instr.GetAccounts()
	if len(accounts) < 3 {
		return nil, nil
	}
	poolIdx := int(accounts[2])
	if poolIdx >= len(accountStrs) {
		return nil, nil
	}
	poolID := accountStrs[poolIdx]
	poolInfo, ok := d.orcaPools[poolID]
	if !ok {
		return nil, nil
	}

	balances := vaults[poolID]
	if len(balances) == 0 {
		return nil, nil
	}

	var vaultA, vaultB *tokenBalance
	for _, tb := range balances {
		if tb.mint == poolInfo.TokenMintA {
			vaultA = tb
		} else if tb.mint == poolInfo.TokenMintB {
			vaultB = tb
		}
	}
	if vaultA == nil || vaultB == nil {
		return nil, nil
	}

	deltaA := int64(vaultA.post) - int64(vaultA.pre)
	deltaB := int64(vaultB.post) - int64(vaultB.pre)
	if deltaA == 0 && deltaB == 0 {
		return nil, nil
	}

	event := &dexv1.SwapEvent{
		ChainId:     chainIDSolana,
		Slot:        slot,
		Sig:         signature,
		Index:       uint32(index),
		ProgramId:   orcawhirlpool.WhirlpoolProgramID,
		PoolId:      poolID,
		MintBase:    poolInfo.TokenMintA,
		MintQuote:   poolInfo.TokenMintB,
		DecBase:     uint32(vaultA.decimals),
		DecQuote:    uint32(vaultB.decimals),
		FeeBps:      uint32(poolInfo.FeeRate / 100),
		Provisional: true,
	}

	if deltaA < 0 && deltaB > 0 {
		event.BaseOut = uint64(-deltaA)
		event.QuoteIn = uint64(deltaB)
	} else {
		if deltaA > 0 {
			event.BaseIn = uint64(deltaA)
		}
		if deltaB < 0 {
			event.QuoteOut = uint64(-deltaB)
		}
	}

	return event, nil
}

func (d *Decoder) buildMeteoraSwap(signature string, slot uint64, timestamp int64, index uint64, instr *pb.CompiledInstruction, accountStrs []string, meta *pb.TransactionStatusMeta, programID string, kind meteora.PoolKind) (*dexv1.SwapEvent, error) {
	ctx := &meteora.InstructionContext{
		Slot:                slot,
		Signature:           signature,
		Accounts:            accountStrs,
		InstructionAccounts: instr.GetAccounts(),
		PreTokenBalances:    meta.GetPreTokenBalances(),
		PostTokenBalances:   meta.GetPostTokenBalances(),
		Logs:                meta.GetLogMessages(),
		ProgramID:           programID,
		Kind:                kind,
	}
	if timestamp != 0 {
		ctx.Timestamp = time.Unix(timestamp, 0)
	}

	event, err := meteora.DecodeSwapEvent(instr.GetData(), ctx)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, nil
	}

	proto := &dexv1.SwapEvent{
		ChainId:     chainIDSolana,
		Slot:        slot,
		Sig:         signature,
		Index:       uint32(index),
		ProgramId:   programID,
		PoolId:      event.Pool,
		MintBase:    event.MintBase,
		MintQuote:   event.MintQuote,
		DecBase:     event.DecBase,
		DecQuote:    event.DecQuote,
		FeeBps:      event.FeeBps,
		Provisional: true,
	}

	resBase := event.VirtualReservesBase
	resQuote := event.VirtualReservesQuote
	if resBase == 0 && resQuote == 0 {
		resBase = event.RealReservesBase
		resQuote = event.RealReservesQuote
	}
	if resBase != 0 || resQuote != 0 {
		proto.ReservesBase = resBase
		proto.ReservesQuote = resQuote
	}

	if event.BaseDecreased {
		proto.BaseOut = event.BaseAmount
		proto.QuoteIn = event.QuoteAmount
	} else {
		proto.BaseIn = event.BaseAmount
		proto.QuoteOut = event.QuoteAmount
	}

	return proto, nil
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
