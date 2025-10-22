package geyser

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	proto "google.golang.org/protobuf/proto"

	meteora "github.com/rexbrahh/lp-indexer/decoder/meteora"
	orcawhirlpool "github.com/rexbrahh/lp-indexer/decoder/orca_whirlpool"
	ray "github.com/rexbrahh/lp-indexer/decoder/raydium"
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
	"github.com/rexbrahh/lp-indexer/ingestor/common"
	swapdecoder "github.com/rexbrahh/lp-indexer/ingestor/decoder"
	"github.com/rexbrahh/lp-indexer/observability"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

const chainIDSolana = 501

// SwapPublisher publishes canonical swap events.
type SwapPublisher interface {
	PublishSwap(ctx context.Context, event *dexv1.SwapEvent) error
	PublishBlockHead(ctx context.Context, head *dexv1.BlockHead) error
	PublishTxMeta(ctx context.Context, meta *dexv1.TxMeta) error
}

// Processor consumes geyser updates and emits canonical swap events.
type Processor struct {
	publisher  SwapPublisher
	decoder    *swapdecoder.Decoder
	metrics    *processorMetrics
	pending    map[uint64][]*dexv1.SwapEvent
	blockHeads map[uint64]*dexv1.BlockHead
}

// NewProcessor initialises a Processor with optional metrics registration.
func NewProcessor(publisher SwapPublisher, cache common.SlotTimeCache, reg prometheus.Registerer) *Processor {
	return &Processor{
		publisher:  publisher,
		decoder:    swapdecoder.New(cache),
		metrics:    newProcessorMetrics(reg),
		pending:    make(map[uint64][]*dexv1.SwapEvent),
		blockHeads: make(map[uint64]*dexv1.BlockHead),
	}
}

// HandleUpdate inspects an incoming geyser update and routes it to the decoder.
func (p *Processor) HandleUpdate(ctx context.Context, update *pb.SubscribeUpdate) error {
	if update == nil {
		return nil
	}

	switch u := update.GetUpdateOneof().(type) {
	case *pb.SubscribeUpdate_Transaction:
		return p.handleTransaction(ctx, u.Transaction)
	case *pb.SubscribeUpdate_BlockMeta:
		return p.handleBlockMeta(ctx, u.BlockMeta)
	case *pb.SubscribeUpdate_Account:
		p.handleAccount(u.Account)
	case *pb.SubscribeUpdate_Slot:
		return p.handleSlot(ctx, u.Slot)
	}
	return nil
}

func (p *Processor) handleTransaction(ctx context.Context, tx *pb.SubscribeUpdateTransaction) error {
	if tx == nil {
		return nil
	}

	events, err := p.decoder.DecodeTransaction(tx)
	if err != nil {
		var decodeErr *swapdecoder.DecodeError
		if errors.As(err, &decodeErr) {
			p.metrics.recordError(decodeErr.Program)
		}
		return fmt.Errorf("decode transaction: %w", err)
	}

	if meta := common.ConvertTxMeta(tx); meta != nil {
		if err := p.publisher.PublishTxMeta(ctx, meta); err != nil {
			return fmt.Errorf("publish tx meta: %w", err)
		}
	}

	for _, ev := range events {
		p.metrics.recordSwap(ev.GetProgramId())
		if err := p.publisher.PublishSwap(ctx, ev); err != nil {
			p.metrics.recordError(ev.GetProgramId())
			return fmt.Errorf("publish swap: %w", err)
		}
		p.appendPending(ev.GetSlot(), ev)
	}
	return nil
}

func (p *Processor) handleBlockMeta(ctx context.Context, meta *pb.SubscribeUpdateBlockMeta) error {
	p.decoder.HandleBlockMeta(meta)
	if meta == nil {
		return nil
	}

	head := &dexv1.BlockHead{
		ChainId: chainIDSolana,
		Slot:    meta.GetSlot(),
		Status:  "confirmed",
	}
	if ts := meta.GetBlockTime(); ts != nil {
		head.TsSec = uint64(ts.GetTimestamp())
	}
	p.blockHeads[head.Slot] = head
	return p.publisher.PublishBlockHead(ctx, proto.Clone(head).(*dexv1.BlockHead))
}

func (p *Processor) handleAccount(account *pb.SubscribeUpdateAccount) {
	p.decoder.HandleAccount(account)
}

func (p *Processor) handleSlot(ctx context.Context, update *pb.SubscribeUpdateSlot) error {
	if update == nil {
		return nil
	}
	slot := update.GetSlot()
	switch update.GetStatus() {
	case pb.SlotStatus_SLOT_FINALIZED:
		if err := p.finalizeSlot(ctx, slot); err != nil {
			return err
		}
		return p.publishBlockHeadStatus(ctx, slot, "finalized")
	case pb.SlotStatus_SLOT_DEAD:
		if err := p.undoSlot(ctx, slot); err != nil {
			return err
		}
		return p.publishBlockHeadStatus(ctx, slot, "dead")
	default:
		return nil
	}
}

func (p *Processor) finalizeSlot(ctx context.Context, slot uint64) error {
	events := p.pending[slot]
	if len(events) == 0 {
		delete(p.pending, slot)
		return nil
	}
	for _, ev := range events {
		final := proto.Clone(ev).(*dexv1.SwapEvent)
		final.Provisional = false
		final.IsUndo = false
		if err := p.publisher.PublishSwap(ctx, final); err != nil {
			p.metrics.recordError(final.GetProgramId())
			return fmt.Errorf("publish finalized swap: %w", err)
		}
	}
	delete(p.pending, slot)
	return nil
}

func (p *Processor) undoSlot(ctx context.Context, slot uint64) error {
	events := p.pending[slot]
	if len(events) == 0 {
		delete(p.pending, slot)
		return nil
	}
	for _, ev := range events {
		undo := proto.Clone(ev).(*dexv1.SwapEvent)
		undo.Provisional = false
		undo.IsUndo = true
		if err := p.publisher.PublishSwap(ctx, undo); err != nil {
			p.metrics.recordError(undo.GetProgramId())
			return fmt.Errorf("publish undo swap: %w", err)
		}
	}
	delete(p.pending, slot)
	return nil
}

func (p *Processor) publishBlockHeadStatus(ctx context.Context, slot uint64, status string) error {
	head, ok := p.blockHeads[slot]
	if !ok {
		return nil
	}
	updated := proto.Clone(head).(*dexv1.BlockHead)
	updated.Status = status
	if err := p.publisher.PublishBlockHead(ctx, updated); err != nil {
		return fmt.Errorf("publish block head %s: %w", status, err)
	}
	head.Status = status
	if status == "finalized" || status == "dead" {
		delete(p.blockHeads, slot)
	}
	return nil
}

func (p *Processor) appendPending(slot uint64, event *dexv1.SwapEvent) {
	if event == nil {
		return
	}
	clone := proto.Clone(event).(*dexv1.SwapEvent)
	p.pending[slot] = append(p.pending[slot], clone)
}

type processorMetrics struct {
	raydiumSwaps  prometheus.Counter
	raydiumErrors prometheus.Counter
	orcaSwaps     prometheus.Counter
	orcaErrors    prometheus.Counter
	meteoraSwaps  prometheus.Counter
	meteoraErrors prometheus.Counter
}

func newProcessorMetrics(reg prometheus.Registerer) *processorMetrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	return &processorMetrics{
		raydiumSwaps: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricRaydiumSwapsTotal,
			Help:      "Total Raydium swaps decoded from geyser transactions.",
		}),
		raydiumErrors: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricRaydiumDecodeErrors,
			Help:      "Raydium swap decode or publish errors.",
		}),
		orcaSwaps: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricOrcaSwapsTotal,
			Help:      "Total Orca swaps decoded from geyser transactions.",
		}),
		orcaErrors: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricOrcaDecodeErrors,
			Help:      "Orca swap decode or publish errors.",
		}),
		meteoraSwaps: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricMeteoraSwapsTotal,
			Help:      "Total Meteora swaps decoded from geyser transactions.",
		}),
		meteoraErrors: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "geyser",
			Name:      observability.MetricMeteoraDecodeErrors,
			Help:      "Meteora swap decode or publish errors.",
		}),
	}
}

func (m *processorMetrics) recordSwap(programID string) {
	if m == nil {
		return
	}
	switch programID {
	case ray.ProgramID:
		m.raydiumSwaps.Inc()
	case orcawhirlpool.WhirlpoolProgramID:
		m.orcaSwaps.Inc()
	default:
		if _, ok := meteora.ProgramKindForID(programID); ok {
			m.meteoraSwaps.Inc()
		}
	}
}

func (m *processorMetrics) recordError(programID string) {
	if m == nil {
		return
	}
	switch programID {
	case ray.ProgramID:
		m.raydiumErrors.Inc()
	case orcawhirlpool.WhirlpoolProgramID:
		m.orcaErrors.Inc()
	default:
		if _, ok := meteora.ProgramKindForID(programID); ok {
			m.meteoraErrors.Inc()
		}
	}
}
