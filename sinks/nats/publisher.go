package natsx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	nats "github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

// Publisher wraps a JetStream connection for emitting canonical protobuf events.
type Publisher struct {
	cfg  Config
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewPublisher dials JetStream using the provided configuration.
func NewPublisher(cfg Config) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := []nats.Option{nats.Name("solana-liquidity-indexer")}
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream context: %w", err)
	}

	return &Publisher{cfg: cfg, conn: conn, js: js}, nil
}

// Close drains and closes the underlying NATS connection.
func (p *Publisher) Close() {
	if p.conn == nil {
		return
	}
	_ = p.conn.Drain()
	p.conn.Close()
}

// PublishSwap publishes a SwapEvent to JetStream.
func (p *Publisher) PublishSwap(ctx context.Context, event *dexv1.SwapEvent) error {
	if event == nil {
		return errors.New("swap event is nil")
	}
	subject := fmt.Sprintf("%s.%s.swap", p.cfg.SubjectRoot, programSegment(event.GetProgramId()))
	msgID := fmt.Sprintf("501:%d:%s:%d", event.GetSlot(), event.GetSig(), event.GetIndex())
	return p.publish(ctx, subject, event, msgID)
}

// PublishPoolSnapshot publishes a PoolSnapshot update.
func (p *Publisher) PublishPoolSnapshot(ctx context.Context, snap *dexv1.PoolSnapshot) error {
	if snap == nil {
		return errors.New("pool snapshot is nil")
	}
	subject := fmt.Sprintf("%s.pool.snapshot", p.cfg.SubjectRoot)
	msgID := fmt.Sprintf("501:%d:%s", snap.GetSlot(), snap.GetPoolId())
	return p.publish(ctx, subject, snap, msgID)
}

// PublishCandle publishes a Candle update (pool or pair scope).
func (p *Publisher) PublishCandle(ctx context.Context, candle *dexv1.Candle) error {
	if candle == nil {
		return errors.New("candle is nil")
	}
	scope := "pair"
	if candle.GetPoolId() != "" {
		scope = "pool"
	}
	subject := fmt.Sprintf("%s.candle.%s.%s", p.cfg.SubjectRoot, scope, candle.GetTimeframe())
	msgID := fmt.Sprintf("501:%s:%s:%d:%t", candle.GetPairId(), candle.GetPoolId(), candle.GetWindowStart(), candle.GetProvisional())
	return p.publish(ctx, subject, candle, msgID)
}

func (p *Publisher) publish(parent context.Context, subject string, message proto.Message, msgID string) error {
	if message == nil {
		return errors.New("message is nil")
	}

	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	msg := &nats.Msg{Subject: subject, Data: data}
	msg.Header = nats.Header{}
	if msgID != "" {
		msg.Header.Set("Nats-Msg-Id", msgID)
	}
	msg.Header.Set("Content-Type", "application/protobuf")

	ctx, cancel := p.ensureTimeout(parent)
	defer cancel()

	ack, err := p.js.PublishMsg(msg, nats.Context(ctx), nats.ExpectStream(p.cfg.Stream))
	if err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	if ack != nil && ack.Stream != "" && ack.Stream != p.cfg.Stream {
		return fmt.Errorf("unexpected stream ack %q (expected %q)", ack.Stream, p.cfg.Stream)
	}
	return nil
}

func (p *Publisher) ensureTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if _, ok := parent.Deadline(); ok || p.cfg.PublishTimeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, p.cfg.PublishTimeout)
}

// Config exposes a copy of the publisher configuration.
func (p *Publisher) Config() Config {
	return p.cfg
}

// WithTimeout exposes the timeout helper for external callers.
func (p *Publisher) WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return p.ensureTimeout(parent)
}

var programSubjectAliases = map[string]string{
	"CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK":  "raydium",
	"whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc":   "orca",
	"METoRa111111111111111111111111111111111111111": "meteora",
}

func programSegment(programID string) string {
	if programID == "" {
		return "unknown"
	}
	if alias, ok := programSubjectAliases[programID]; ok {
		return alias
	}
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '.', ' ', '*', '>':
			return -1
		}
		return r
	}, programID)
	if len(cleaned) > 16 {
		cleaned = cleaned[:16]
	}
	return strings.ToLower(cleaned)
}
