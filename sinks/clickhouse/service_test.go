package clickhouse

import (
	"context"
	"testing"
	"time"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

type stubWriter struct {
	trades []Trade
	flush  int
}

func (s *stubWriter) WriteTrades(_ context.Context, trades []Trade) error {
	s.trades = append(s.trades, trades...)
	return nil
}

func (s *stubWriter) Flush(_ context.Context) error {
	s.flush++
	return nil
}

func TestProcessorHandlesBlockHeadAndSwap(t *testing.T) {
	writer := &stubWriter{}
	proc := newProcessor(writer)

	head := &dexv1.BlockHead{
		ChainId: 501,
		Slot:    123,
		TsSec:   1_700_000_000,
		Status:  "confirmed",
	}
	proc.handleBlockHead(head)

	swap := &dexv1.SwapEvent{
		ChainId:       501,
		Slot:          123,
		Sig:           "sig",
		Index:         2,
		ProgramId:     "prog",
		PoolId:        "pool",
		MintBase:      "base",
		MintQuote:     "quote",
		DecBase:       6,
		DecQuote:      6,
		BaseIn:        10,
		QuoteOut:      20,
		FeeBps:        30,
		ReservesBase:  1000,
		ReservesQuote: 2000,
		Provisional:   true,
	}

	if err := proc.handleSwap(context.Background(), swap); err != nil {
		t.Fatalf("handleSwap error: %v", err)
	}

	if len(writer.trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(writer.trades))
	}
	trade := writer.trades[0]
	if trade.Signature != "sig" || trade.Index != 2 {
		t.Fatalf("unexpected trade fields: %+v", trade)
	}
	if trade.Timestamp != time.Unix(1_700_000_000, 0).UTC() {
		t.Fatalf("unexpected timestamp %v", trade.Timestamp)
	}
	if !trade.Provisional {
		t.Fatal("expected provisional flag")
	}
	if trade.IsUndo {
		t.Fatal("did not expect undo flag")
	}
	if trade.ReservesBase != 1000 || trade.ReservesQuote != 2000 {
		t.Fatalf("unexpected reserves %+v", trade)
	}
}

func TestProcessorHandlesUndo(t *testing.T) {
	writer := &stubWriter{}
	proc := newProcessor(writer)

	proc.handleBlockHead(&dexv1.BlockHead{Slot: 99, TsSec: 1})

	swap := &dexv1.SwapEvent{
		ChainId:     501,
		Slot:        99,
		Sig:         "undo",
		Index:       1,
		PoolId:      "pool",
		Provisional: false,
		IsUndo:      true,
	}

	if err := proc.handleSwap(context.Background(), swap); err != nil {
		t.Fatalf("handleSwap undo error: %v", err)
	}
	if len(writer.trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(writer.trades))
	}
	if !writer.trades[0].IsUndo {
		t.Fatal("expected undo trade")
	}
}
