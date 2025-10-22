package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	proto "google.golang.org/protobuf/proto"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

type tradeWriter interface {
	WriteTrades(ctx context.Context, trades []Trade) error
	Flush(ctx context.Context) error
}

type processor struct {
	writer    tradeWriter
	slotTimes map[uint64]time.Time
}

func newProcessor(writer tradeWriter) *processor {
	return &processor{
		writer:    writer,
		slotTimes: make(map[uint64]time.Time),
	}
}

func (p *processor) handleBlockHead(head *dexv1.BlockHead) {
	if head == nil {
		return
	}
	ts := time.Unix(int64(head.GetTsSec()), 0).UTC()
	if ts.IsZero() {
		return
	}
	p.slotTimes[head.GetSlot()] = ts
	status := strings.ToLower(head.GetStatus())
	if status == "dead" {
		delete(p.slotTimes, head.GetSlot())
	}
}

func (p *processor) handleSwap(ctx context.Context, event *dexv1.SwapEvent) error {
	if event == nil {
		return nil
	}
	ts := p.slotTimes[event.GetSlot()]
	trade := Trade{
		ChainID:       uint16(event.GetChainId()),
		Slot:          event.GetSlot(),
		Timestamp:     ts,
		Signature:     event.GetSig(),
		Index:         event.GetIndex(),
		ProgramID:     event.GetProgramId(),
		PoolID:        event.GetPoolId(),
		MintBase:      event.GetMintBase(),
		MintQuote:     event.GetMintQuote(),
		DecBase:       uint8(event.GetDecBase()),
		DecQuote:      uint8(event.GetDecQuote()),
		BaseIn:        event.GetBaseIn(),
		BaseOut:       event.GetBaseOut(),
		QuoteIn:       event.GetQuoteIn(),
		QuoteOut:      event.GetQuoteOut(),
		PriceQ32:      0,
		ReservesBase:  event.GetReservesBase(),
		ReservesQuote: event.GetReservesQuote(),
		FeeBps:        uint16(event.GetFeeBps()),
		Provisional:   event.GetProvisional(),
		IsUndo:        event.GetIsUndo(),
	}
	return p.writer.WriteTrades(ctx, []Trade{trade})
}

type Service struct {
	cfg       ServiceConfig
	conn      *nats.Conn
	js        nats.JetStreamContext
	sub       *nats.Subscription
	processor *processor
	lastFlush time.Time
}

func NewService(ctx context.Context, cfg ServiceConfig) (*Service, error) {
	writer, err := NewWithConfig(ctx, cfg.Writer)
	if err != nil {
		return nil, err
	}

	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	subject := cfg.SubjectRoot + ".>"
	sub, err := js.PullSubscribe(subject, cfg.Consumer, nats.BindStream(cfg.Stream), nats.ManualAck())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("pull subscribe: %w", err)
	}

	return &Service{
		cfg:       cfg,
		conn:      conn,
		js:        js,
		sub:       sub,
		processor: newProcessor(writer),
		lastFlush: time.Now(),
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	flushTicker := time.NewTicker(s.cfg.Writer.FlushInterval)
	defer flushTicker.Stop()
	defer s.conn.Drain()
	defer s.processor.writer.Flush(context.Background())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-flushTicker.C:
			if err := s.processor.writer.Flush(ctx); err != nil {
				return err
			}
		default:
		}

		msgs, err := s.sub.Fetch(s.cfg.PullBatch, nats.MaxWait(s.cfg.PullTimeout))
		if errors.Is(err, nats.ErrTimeout) {
			continue
		}
		if err != nil {
			return fmt.Errorf("fetch messages: %w", err)
		}

		for _, msg := range msgs {
			if err := s.handleMessage(ctx, msg); err != nil {
				_ = msg.Nak()
				return err
			}
			_ = msg.Ack()
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg *nats.Msg) error {
	subject := msg.Subject
	switch {
	case strings.HasSuffix(subject, ".swap"):
		var event dexv1.SwapEvent
		if err := proto.Unmarshal(msg.Data, &event); err != nil {
			return fmt.Errorf("unmarshal swap: %w", err)
		}
		return s.processor.handleSwap(ctx, &event)
	case strings.HasSuffix(subject, ".blocks.head"):
		var head dexv1.BlockHead
		if err := proto.Unmarshal(msg.Data, &head); err != nil {
			return fmt.Errorf("unmarshal block head: %w", err)
		}
		s.processor.handleBlockHead(&head)
		return nil
	case strings.HasSuffix(subject, ".tx.meta"):
		// Tx meta currently unused but must be acked.
		return nil
	default:
		return nil
	}
}
