package clickhouse

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"
)

// Config holds ClickHouse writer configuration
type Config struct {
	DSN              string
	Database         string
	TradesTable      string
	CandlesTable     string
	BatchSize        int
	FlushInterval    time.Duration
	MaxRetries       int
	RetryBackoffBase time.Duration
	RetryBackoffMax  time.Duration
}

// Writer manages ClickHouse connections and batch writes
type Writer struct {
	config Config
	client *ch.Client

	tradesBatch  *tradeBatch
	candlesBatch *candleBatch
}

type tradeBatch struct {
	slots      proto.ColUInt64
	signatures proto.ColStr
	blockTimes proto.ColDateTime64
	poolIDs    proto.ColStr
	amounts    proto.ColFloat64
	count      int
}

type candleBatch struct {
	timestamps proto.ColDateTime64
	poolIDs    proto.ColStr
	opens      proto.ColFloat64
	highs      proto.ColFloat64
	lows       proto.ColFloat64
	closes     proto.ColFloat64
	volumes    proto.ColFloat64
	count      int
}

// NewWithConfig creates a new ClickHouse writer with the given configuration
func NewWithConfig(ctx context.Context, cfg Config) (*Writer, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client, err := connectWithRetry(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	blockTimes := proto.ColDateTime64{}
	blockTimes.WithPrecision(proto.PrecisionNano)

	timestamps := proto.ColDateTime64{}
	timestamps.WithPrecision(proto.PrecisionNano)

	w := &Writer{
		config: cfg,
		client: client,
		tradesBatch: &tradeBatch{
			slots:      proto.ColUInt64{},
			signatures: proto.ColStr{},
			blockTimes: blockTimes,
			poolIDs:    proto.ColStr{},
			amounts:    proto.ColFloat64{},
		},
		candlesBatch: &candleBatch{
			timestamps: timestamps,
			poolIDs:    proto.ColStr{},
			opens:      proto.ColFloat64{},
			highs:      proto.ColFloat64{},
			lows:       proto.ColFloat64{},
			closes:     proto.ColFloat64{},
			volumes:    proto.ColFloat64{},
		},
	}

	return w, nil
}

// validateConfig checks that required configuration fields are set
func validateConfig(cfg Config) error {
	if cfg.DSN == "" {
		return fmt.Errorf("dsn is required")
	}
	if cfg.Database == "" {
		return fmt.Errorf("database is required")
	}
	if cfg.TradesTable == "" {
		return fmt.Errorf("trades table is required")
	}
	if cfg.CandlesTable == "" {
		return fmt.Errorf("candles table is required")
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}
	return nil
}

// connectWithRetry attempts to connect to ClickHouse with exponential backoff
func connectWithRetry(ctx context.Context, cfg Config) (*ch.Client, error) {
	opts, err := parseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	opts.Database = cfg.Database

	var client *ch.Client
	backoff := cfg.RetryBackoffBase
	if backoff == 0 {
		backoff = 100 * time.Millisecond
	}

	maxBackoff := cfg.RetryBackoffMax
	if maxBackoff == 0 {
		maxBackoff = 10 * time.Second
	}

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		client, err = ch.Dial(ctx, opts)
		if err == nil {
			return client, nil
		}

		if attempt < cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", cfg.MaxRetries, err)
}

// parseDSN parses a ClickHouse DSN and returns client options
// Format: clickhouse://user:password@host:port/database?param=value
func parseDSN(dsn string) (ch.Options, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return ch.Options{}, fmt.Errorf("invalid DSN format: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "clickhouse", "tcp":
		// Accept both modern clickhouse:// and historical tcp:// prefixes.
	case "":
		return ch.Options{}, fmt.Errorf("invalid scheme: expected 'clickhouse' or 'tcp'")
	default:
		return ch.Options{}, fmt.Errorf("invalid scheme: expected 'clickhouse' or 'tcp', got '%s'", u.Scheme)
	}

	opts := ch.Options{
		Address: u.Host,
	}

	if u.User != nil {
		opts.User = u.User.Username()
		if password, ok := u.User.Password(); ok {
			opts.Password = password
		}
	}

	// Extract database from path if present
	if len(u.Path) > 1 {
		opts.Database = u.Path[1:] // Skip leading '/'
	}

	// Parse query parameters for additional options
	query := u.Query()
	if compression := query.Get("compression"); compression != "" {
		switch compression {
		case "lz4":
			opts.Compression = ch.CompressionLZ4
		case "none":
			opts.Compression = ch.CompressionNone
		}
	}

	return opts, nil
}

// Trade represents a single DEX swap event
type Trade struct {
	Slot      uint64
	Signature string
	BlockTime time.Time
	PoolID    string
	Amount    float64
}

// WriteTrades adds trades to the batch and flushes if batch size is reached
func (w *Writer) WriteTrades(ctx context.Context, trades []Trade) error {
	for _, trade := range trades {
		w.tradesBatch.slots.Append(trade.Slot)
		w.tradesBatch.signatures.Append(trade.Signature)
		w.tradesBatch.blockTimes.Append(trade.BlockTime)
		w.tradesBatch.poolIDs.Append(trade.PoolID)
		w.tradesBatch.amounts.Append(trade.Amount)
		w.tradesBatch.count++

		if w.tradesBatch.count >= w.config.BatchSize {
			if err := w.flushTrades(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// Candle represents OHLCV candle data
type Candle struct {
	Timestamp time.Time
	PoolID    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// WriteCandles adds candles to the batch and flushes if batch size is reached
func (w *Writer) WriteCandles(ctx context.Context, candles []Candle) error {
	for _, candle := range candles {
		w.candlesBatch.timestamps.Append(candle.Timestamp)
		w.candlesBatch.poolIDs.Append(candle.PoolID)
		w.candlesBatch.opens.Append(candle.Open)
		w.candlesBatch.highs.Append(candle.High)
		w.candlesBatch.lows.Append(candle.Low)
		w.candlesBatch.closes.Append(candle.Close)
		w.candlesBatch.volumes.Append(candle.Volume)
		w.candlesBatch.count++

		if w.candlesBatch.count >= w.config.BatchSize {
			if err := w.flushCandles(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// flushTrades writes the current trade batch to ClickHouse
func (w *Writer) flushTrades(ctx context.Context) error {
	if w.tradesBatch.count == 0 {
		return nil
	}

	input := proto.Input{
		{Name: "slot", Data: w.tradesBatch.slots},
		{Name: "signature", Data: w.tradesBatch.signatures},
		{Name: "block_time", Data: w.tradesBatch.blockTimes},
		{Name: "pool_id", Data: w.tradesBatch.poolIDs},
		{Name: "amount", Data: w.tradesBatch.amounts},
	}

	if err := w.client.Do(ctx, ch.Query{
		Body:  fmt.Sprintf("INSERT INTO %s VALUES", w.config.TradesTable),
		Input: input,
	}); err != nil {
		return fmt.Errorf("failed to flush trades: %w", err)
	}

	// Reset batch
	w.tradesBatch.slots = proto.ColUInt64{}
	w.tradesBatch.signatures = proto.ColStr{}
	blockTimes := proto.ColDateTime64{}
	blockTimes.WithPrecision(proto.PrecisionNano)
	w.tradesBatch.blockTimes = blockTimes
	w.tradesBatch.poolIDs = proto.ColStr{}
	w.tradesBatch.amounts = proto.ColFloat64{}
	w.tradesBatch.count = 0

	return nil
}

// flushCandles writes the current candle batch to ClickHouse
func (w *Writer) flushCandles(ctx context.Context) error {
	if w.candlesBatch.count == 0 {
		return nil
	}

	input := proto.Input{
		{Name: "timestamp", Data: w.candlesBatch.timestamps},
		{Name: "pool_id", Data: w.candlesBatch.poolIDs},
		{Name: "open", Data: w.candlesBatch.opens},
		{Name: "high", Data: w.candlesBatch.highs},
		{Name: "low", Data: w.candlesBatch.lows},
		{Name: "close", Data: w.candlesBatch.closes},
		{Name: "volume", Data: w.candlesBatch.volumes},
	}

	if err := w.client.Do(ctx, ch.Query{
		Body:  fmt.Sprintf("INSERT INTO %s VALUES", w.config.CandlesTable),
		Input: input,
	}); err != nil {
		return fmt.Errorf("failed to flush candles: %w", err)
	}

	// Reset batch
	timestamps := proto.ColDateTime64{}
	timestamps.WithPrecision(proto.PrecisionNano)
	w.candlesBatch.timestamps = timestamps
	w.candlesBatch.poolIDs = proto.ColStr{}
	w.candlesBatch.opens = proto.ColFloat64{}
	w.candlesBatch.highs = proto.ColFloat64{}
	w.candlesBatch.lows = proto.ColFloat64{}
	w.candlesBatch.closes = proto.ColFloat64{}
	w.candlesBatch.volumes = proto.ColFloat64{}
	w.candlesBatch.count = 0

	return nil
}

// Flush writes any remaining batched data to ClickHouse
func (w *Writer) Flush(ctx context.Context) error {
	if err := w.flushTrades(ctx); err != nil {
		return err
	}
	return w.flushCandles(ctx)
}

// Close flushes remaining data and closes the connection
func (w *Writer) Close(ctx context.Context) error {
	if err := w.Flush(ctx); err != nil {
		return err
	}
	return w.client.Close()
}
