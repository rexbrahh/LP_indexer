package parquet

import (
	"context"
	"errors"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

// ErrNotImplemented indicates the Parquet writer has not been wired yet.
var ErrNotImplemented = errors.New("parquet writer not yet implemented")

// Writer buffers protobuf messages and flushes them to cold storage.
type Writer struct {
	cfg Config
}

// NewWriter validates configuration and prepares a Writer.
func NewWriter(cfg Config) (*Writer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Writer{cfg: cfg}, nil
}

// AppendSwap buffers a swap event for inclusion in a Parquet batch.
func (w *Writer) AppendSwap(ctx context.Context, event *dexv1.SwapEvent) error {
	return w.append(ctx, event)
}

// AppendCandle buffers a candle event.
func (w *Writer) AppendCandle(ctx context.Context, candle *dexv1.Candle) error {
	return w.append(ctx, candle)
}

// Flush forces buffered records to be written.
func (w *Writer) Flush(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

func (w *Writer) append(ctx context.Context, payload any) error {
	_ = ctx
	_ = payload
	return ErrNotImplemented
}

// Config returns a copy of the writer configuration.
func (w *Writer) Config() Config {
	return w.cfg
}
