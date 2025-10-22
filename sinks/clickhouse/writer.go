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
	chainIDs      proto.ColUInt16
	slots         proto.ColUInt64
	timestamps    proto.ColDateTime64
	signatures    proto.ColStr
	indices       proto.ColUInt32
	programIDs    proto.ColStr
	pools         proto.ColStr
	mintBase      proto.ColStr
	mintQuote     proto.ColStr
	decBase       proto.ColUInt8
	decQuote      proto.ColUInt8
	baseIn        proto.ColDecimal128
	baseOut       proto.ColDecimal128
	quoteIn       proto.ColDecimal128
	quoteOut      proto.ColDecimal128
	priceQ32      proto.ColInt64
	reservesBase  proto.ColDecimal128
	reservesQuote proto.ColDecimal128
	feeBps        proto.ColUInt16
	provisional   proto.ColUInt8
	isUndo        proto.ColUInt8
	count         int
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
	blockTimes.WithPrecision(proto.PrecisionMilli)

	timestamps := proto.ColDateTime64{}
	timestamps.WithPrecision(proto.PrecisionMilli)

	w := &Writer{
		config: cfg,
		client: client,
		tradesBatch: &tradeBatch{
			chainIDs:      proto.ColUInt16{},
			slots:         proto.ColUInt64{},
			timestamps:    blockTimes,
			signatures:    proto.ColStr{},
			indices:       proto.ColUInt32{},
			programIDs:    proto.ColStr{},
			pools:         proto.ColStr{},
			mintBase:      proto.ColStr{},
			mintQuote:     proto.ColStr{},
			decBase:       proto.ColUInt8{},
			decQuote:      proto.ColUInt8{},
			baseIn:        proto.ColDecimal128{},
			baseOut:       proto.ColDecimal128{},
			quoteIn:       proto.ColDecimal128{},
			quoteOut:      proto.ColDecimal128{},
			priceQ32:      proto.ColInt64{},
			reservesBase:  proto.ColDecimal128{},
			reservesQuote: proto.ColDecimal128{},
			feeBps:        proto.ColUInt16{},
			provisional:   proto.ColUInt8{},
			isUndo:        proto.ColUInt8{},
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
	ChainID       uint16
	Slot          uint64
	Timestamp     time.Time
	Signature     string
	Index         uint32
	ProgramID     string
	PoolID        string
	MintBase      string
	MintQuote     string
	DecBase       uint8
	DecQuote      uint8
	BaseIn        uint64
	BaseOut       uint64
	QuoteIn       uint64
	QuoteOut      uint64
	PriceQ32      int64
	ReservesBase  uint64
	ReservesQuote uint64
	FeeBps        uint16
	Provisional   bool
	IsUndo        bool
}

// WriteTrades adds trades to the batch and flushes if batch size is reached
func (w *Writer) WriteTrades(ctx context.Context, trades []Trade) error {
	for _, trade := range trades {
		w.tradesBatch.chainIDs.Append(trade.ChainID)
		w.tradesBatch.slots.Append(trade.Slot)
		w.tradesBatch.timestamps.Append(trade.Timestamp)
		w.tradesBatch.signatures.Append(trade.Signature)
		w.tradesBatch.indices.Append(trade.Index)
		w.tradesBatch.programIDs.Append(trade.ProgramID)
		w.tradesBatch.pools.Append(trade.PoolID)
		w.tradesBatch.mintBase.Append(trade.MintBase)
		w.tradesBatch.mintQuote.Append(trade.MintQuote)
		w.tradesBatch.decBase.Append(trade.DecBase)
		w.tradesBatch.decQuote.Append(trade.DecQuote)
		w.tradesBatch.baseIn.Append(decimal128FromUint64(trade.BaseIn))
		w.tradesBatch.baseOut.Append(decimal128FromUint64(trade.BaseOut))
		w.tradesBatch.quoteIn.Append(decimal128FromUint64(trade.QuoteIn))
		w.tradesBatch.quoteOut.Append(decimal128FromUint64(trade.QuoteOut))
		w.tradesBatch.priceQ32.Append(trade.PriceQ32)
		w.tradesBatch.reservesBase.Append(decimal128FromUint64(trade.ReservesBase))
		w.tradesBatch.reservesQuote.Append(decimal128FromUint64(trade.ReservesQuote))
		w.tradesBatch.feeBps.Append(trade.FeeBps)
		if trade.Provisional {
			w.tradesBatch.provisional.Append(1)
		} else {
			w.tradesBatch.provisional.Append(0)
		}
		if trade.IsUndo {
			w.tradesBatch.isUndo.Append(1)
		} else {
			w.tradesBatch.isUndo.Append(0)
		}
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
		{Name: "chain_id", Data: w.tradesBatch.chainIDs},
		{Name: "slot", Data: w.tradesBatch.slots},
		{Name: "ts", Data: w.tradesBatch.timestamps},
		{Name: "sig", Data: w.tradesBatch.signatures},
		{Name: "idx", Data: w.tradesBatch.indices},
		{Name: "program_id", Data: w.tradesBatch.programIDs},
		{Name: "pool_id", Data: w.tradesBatch.pools},
		{Name: "mint_base", Data: w.tradesBatch.mintBase},
		{Name: "mint_quote", Data: w.tradesBatch.mintQuote},
		{Name: "dec_base", Data: w.tradesBatch.decBase},
		{Name: "dec_quote", Data: w.tradesBatch.decQuote},
		{Name: "base_in", Data: w.tradesBatch.baseIn},
		{Name: "base_out", Data: w.tradesBatch.baseOut},
		{Name: "quote_in", Data: w.tradesBatch.quoteIn},
		{Name: "quote_out", Data: w.tradesBatch.quoteOut},
		{Name: "price_q32", Data: w.tradesBatch.priceQ32},
		{Name: "reserves_base", Data: w.tradesBatch.reservesBase},
		{Name: "reserves_quote", Data: w.tradesBatch.reservesQuote},
		{Name: "fee_bps", Data: w.tradesBatch.feeBps},
		{Name: "provisional", Data: w.tradesBatch.provisional},
		{Name: "is_undo", Data: w.tradesBatch.isUndo},
	}

	if err := w.client.Do(ctx, ch.Query{
		Body:  fmt.Sprintf("INSERT INTO %s VALUES", w.config.TradesTable),
		Input: input,
	}); err != nil {
		return fmt.Errorf("failed to flush trades: %w", err)
	}

	// Reset batch
	w.tradesBatch.chainIDs = proto.ColUInt16{}
	w.tradesBatch.slots = proto.ColUInt64{}
	timestamps := proto.ColDateTime64{}
	timestamps.WithPrecision(proto.PrecisionNano)
	w.tradesBatch.timestamps = timestamps
	w.tradesBatch.signatures = proto.ColStr{}
	w.tradesBatch.indices = proto.ColUInt32{}
	w.tradesBatch.programIDs = proto.ColStr{}
	w.tradesBatch.pools = proto.ColStr{}
	w.tradesBatch.mintBase = proto.ColStr{}
	w.tradesBatch.mintQuote = proto.ColStr{}
	w.tradesBatch.decBase = proto.ColUInt8{}
	w.tradesBatch.decQuote = proto.ColUInt8{}
	w.tradesBatch.baseIn = proto.ColDecimal128{}
	w.tradesBatch.baseOut = proto.ColDecimal128{}
	w.tradesBatch.quoteIn = proto.ColDecimal128{}
	w.tradesBatch.quoteOut = proto.ColDecimal128{}
	w.tradesBatch.priceQ32 = proto.ColInt64{}
	w.tradesBatch.reservesBase = proto.ColDecimal128{}
	w.tradesBatch.reservesQuote = proto.ColDecimal128{}
	w.tradesBatch.feeBps = proto.ColUInt16{}
	w.tradesBatch.provisional = proto.ColUInt8{}
	w.tradesBatch.isUndo = proto.ColUInt8{}
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

func decimal128FromUint64(v uint64) proto.Decimal128 {
	return proto.Decimal128(proto.Int128FromUInt64(v))
}

// Close flushes remaining data and closes the connection
func (w *Writer) Close(ctx context.Context) error {
	if err := w.Flush(ctx); err != nil {
		return err
	}
	return w.client.Close()
}
