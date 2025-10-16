package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"math"

	"golang.org/x/sync/errgroup"
)

// Range represents an inclusive-exclusive slot window to process via Substreams.
type Range struct {
	StartSlot uint64
	EndSlot   uint64
}

// valid reports whether the range contains at least one slot.
func (r Range) valid() bool {
	return r.EndSlot > r.StartSlot
}

// RangeProcessor executes backfill work for a specific slot window.
type RangeProcessor func(context.Context, Range) error

// Orchestrator coordinates Substreams backfills and sink writes.
type Orchestrator struct {
	cfg       Config
	processor RangeProcessor
}

// New creates an orchestrator with the provided configuration and range processor.
func New(cfg Config, processor RangeProcessor) (*Orchestrator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if processor == nil {
		return nil, errors.New("range processor must not be nil")
	}
	return &Orchestrator{cfg: cfg, processor: processor}, nil
}

// Run executes the backfill process for the configured slot range.
func (o *Orchestrator) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	g, ctx := errgroup.WithContext(ctx)
	workCh := make(chan Range)

	// Producer goroutine: slice the configured slot span into batches.
	g.Go(func() error {
		defer close(workCh)

		start := o.cfg.StartSlot
		limit := o.cfg.EndSlot
		hasLimit := limit != 0

		for {
			if err := ctx.Err(); err != nil {
				return err
			}

			if hasLimit && start >= limit {
				return nil
			}

			batchEnd, overflow := addWithOverflow(start, o.cfg.BatchSize)
			if hasLimit && batchEnd > limit {
				batchEnd = limit
			}

			rng := Range{StartSlot: start, EndSlot: batchEnd}
			if !rng.valid() {
				if overflow {
					// We reached the end of uint64 range; nothing more to schedule.
					return nil
				}
				return fmt.Errorf("invalid range produced: start=%d end=%d", start, batchEnd)
			}

			select {
			case workCh <- rng:
			case <-ctx.Done():
				return ctx.Err()
			}

			start = batchEnd

			if !hasLimit && overflow {
				// When there is no explicit limit we terminate once uint64 wraps.
				return nil
			}
		}
	})

	// Worker goroutines: process ranges with configured concurrency.
	for i := 0; i < o.cfg.Concurrency; i++ {
		g.Go(func() error {
			for rng := range workCh {
				if err := o.processor(ctx, rng); err != nil {
					return fmt.Errorf("range %d-%d: %w", rng.StartSlot, rng.EndSlot, err)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return err
	}
	return nil
}

// Config exposes a copy of the orchestrator config.
func (o *Orchestrator) Config() Config {
	return o.cfg
}

func addWithOverflow(start uint64, delta uint64) (uint64, bool) {
	if delta == 0 {
		return start, false
	}
	if math.MaxUint64-start < delta {
		return math.MaxUint64, true
	}
	return start + delta, false
}
