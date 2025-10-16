package clickhouse

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				DSN:          "clickhouse://localhost:9000/test",
				Database:     "trades_db",
				TradesTable:  "trades",
				CandlesTable: "candles",
				BatchSize:    1000,
				MaxRetries:   3,
			},
			wantErr: false,
		},
		{
			name: "missing DSN",
			config: Config{
				Database:     "trades_db",
				TradesTable:  "trades",
				CandlesTable: "candles",
				BatchSize:    1000,
			},
			wantErr: true,
			errMsg:  "dsn is required",
		},
		{
			name: "missing database",
			config: Config{
				DSN:          "clickhouse://localhost:9000/test",
				TradesTable:  "trades",
				CandlesTable: "candles",
				BatchSize:    1000,
			},
			wantErr: true,
			errMsg:  "database is required",
		},
		{
			name: "missing trades table",
			config: Config{
				DSN:          "clickhouse://localhost:9000/test",
				Database:     "trades_db",
				CandlesTable: "candles",
				BatchSize:    1000,
			},
			wantErr: true,
			errMsg:  "trades table is required",
		},
		{
			name: "missing candles table",
			config: Config{
				DSN:         "clickhouse://localhost:9000/test",
				Database:    "trades_db",
				TradesTable: "trades",
				BatchSize:   1000,
			},
			wantErr: true,
			errMsg:  "candles table is required",
		},
		{
			name: "invalid batch size",
			config: Config{
				DSN:          "clickhouse://localhost:9000/test",
				Database:     "trades_db",
				TradesTable:  "trades",
				CandlesTable: "candles",
				BatchSize:    0,
			},
			wantErr: true,
			errMsg:  "batch size must be positive",
		},
		{
			name: "negative max retries",
			config: Config{
				DSN:          "clickhouse://localhost:9000/test",
				Database:     "trades_db",
				TradesTable:  "trades",
				CandlesTable: "candles",
				BatchSize:    1000,
				MaxRetries:   -1,
			},
			wantErr: true,
			errMsg:  "max retries must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConfig() expected error but got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("validateConfig() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		wantErr  bool
		wantAddr string
		wantUser string
		wantDB   string
	}{
		{
			name:     "basic DSN",
			dsn:      "clickhouse://localhost:9000/testdb",
			wantErr:  false,
			wantAddr: "localhost:9000",
			wantDB:   "testdb",
		},
		{
			name:     "DSN with credentials",
			dsn:      "clickhouse://user:pass@localhost:9000/testdb",
			wantErr:  false,
			wantAddr: "localhost:9000",
			wantUser: "user",
			wantDB:   "testdb",
		},
		{
			name:     "DSN without database",
			dsn:      "clickhouse://localhost:9000",
			wantErr:  false,
			wantAddr: "localhost:9000",
		},
		{
			name:    "invalid scheme",
			dsn:     "postgres://localhost:5432/testdb",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			dsn:     "not a valid url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDSN() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseDSN() unexpected error = %v", err)
				return
			}

			if opts.Address != tt.wantAddr {
				t.Errorf("parseDSN() Address = %v, want %v", opts.Address, tt.wantAddr)
			}

			if tt.wantUser != "" && opts.User != tt.wantUser {
				t.Errorf("parseDSN() User = %v, want %v", opts.User, tt.wantUser)
			}

			if tt.wantDB != "" && opts.Database != tt.wantDB {
				t.Errorf("parseDSN() Database = %v, want %v", opts.Database, tt.wantDB)
			}
		})
	}
}

// mockWriter simulates Writer for testing batch behavior without actual CH connection
type mockWriter struct {
	config       Config
	tradesBatch  *tradeBatch
	candlesBatch *candleBatch
	flushCalls   []string
}

func newMockWriter(cfg Config) *mockWriter {
	return &mockWriter{
		config: cfg,
		tradesBatch: &tradeBatch{
			count: 0,
		},
		candlesBatch: &candleBatch{
			count: 0,
		},
		flushCalls: []string{},
	}
}

func (m *mockWriter) WriteTrades(ctx context.Context, trades []Trade) error {
	for range trades {
		m.tradesBatch.count++
		if m.tradesBatch.count >= m.config.BatchSize {
			m.flushCalls = append(m.flushCalls, fmt.Sprintf("INSERT INTO %s VALUES", m.config.TradesTable))
			m.tradesBatch.count = 0
		}
	}
	return nil
}

func (m *mockWriter) WriteCandles(ctx context.Context, candles []Candle) error {
	for range candles {
		m.candlesBatch.count++
		if m.candlesBatch.count >= m.config.BatchSize {
			m.flushCalls = append(m.flushCalls, fmt.Sprintf("INSERT INTO %s VALUES", m.config.CandlesTable))
			m.candlesBatch.count = 0
		}
	}
	return nil
}

func TestWriteTrades_BatchFlush(t *testing.T) {
	cfg := Config{
		DSN:          "clickhouse://localhost:9000/test",
		Database:     "trades_db",
		TradesTable:  "trades",
		CandlesTable: "candles",
		BatchSize:    3,
		MaxRetries:   3,
	}

	mock := newMockWriter(cfg)
	ctx := context.Background()

	// Test with exactly batch size trades
	trades := []Trade{
		{Slot: 100, Signature: "sig1", BlockTime: time.Now(), PoolID: "pool1", Amount: 1.0},
		{Slot: 101, Signature: "sig2", BlockTime: time.Now(), PoolID: "pool1", Amount: 2.0},
		{Slot: 102, Signature: "sig3", BlockTime: time.Now(), PoolID: "pool1", Amount: 3.0},
	}

	err := mock.WriteTrades(ctx, trades)
	if err != nil {
		t.Errorf("WriteTrades() unexpected error = %v", err)
	}

	if len(mock.flushCalls) != 1 {
		t.Errorf("WriteTrades() expected 1 flush call, got %d", len(mock.flushCalls))
	}

	expectedSQL := "INSERT INTO trades VALUES"
	if len(mock.flushCalls) > 0 && mock.flushCalls[0] != expectedSQL {
		t.Errorf("WriteTrades() flush SQL = %v, want %v", mock.flushCalls[0], expectedSQL)
	}
}

func TestWriteTrades_MultipleBatches(t *testing.T) {
	cfg := Config{
		DSN:          "clickhouse://localhost:9000/test",
		Database:     "trades_db",
		TradesTable:  "trades",
		CandlesTable: "candles",
		BatchSize:    2,
		MaxRetries:   3,
	}

	mock := newMockWriter(cfg)
	ctx := context.Background()

	// Test with 5 trades (should trigger 2 flushes)
	trades := []Trade{
		{Slot: 100, Signature: "sig1", BlockTime: time.Now(), PoolID: "pool1", Amount: 1.0},
		{Slot: 101, Signature: "sig2", BlockTime: time.Now(), PoolID: "pool1", Amount: 2.0},
		{Slot: 102, Signature: "sig3", BlockTime: time.Now(), PoolID: "pool1", Amount: 3.0},
		{Slot: 103, Signature: "sig4", BlockTime: time.Now(), PoolID: "pool1", Amount: 4.0},
		{Slot: 104, Signature: "sig5", BlockTime: time.Now(), PoolID: "pool1", Amount: 5.0},
	}

	err := mock.WriteTrades(ctx, trades)
	if err != nil {
		t.Errorf("WriteTrades() unexpected error = %v", err)
	}

	if len(mock.flushCalls) != 2 {
		t.Errorf("WriteTrades() expected 2 flush calls, got %d", len(mock.flushCalls))
	}
}

func TestWriteCandles_BatchFlush(t *testing.T) {
	cfg := Config{
		DSN:          "clickhouse://localhost:9000/test",
		Database:     "trades_db",
		TradesTable:  "trades",
		CandlesTable: "candles_1m",
		BatchSize:    3,
		MaxRetries:   3,
	}

	mock := newMockWriter(cfg)
	ctx := context.Background()

	// Test with exactly batch size candles
	candles := []Candle{
		{Timestamp: time.Now(), PoolID: "pool1", Open: 1.0, High: 2.0, Low: 0.5, Close: 1.5, Volume: 100.0},
		{Timestamp: time.Now(), PoolID: "pool1", Open: 1.5, High: 2.5, Low: 1.0, Close: 2.0, Volume: 150.0},
		{Timestamp: time.Now(), PoolID: "pool1", Open: 2.0, High: 3.0, Low: 1.5, Close: 2.5, Volume: 200.0},
	}

	err := mock.WriteCandles(ctx, candles)
	if err != nil {
		t.Errorf("WriteCandles() unexpected error = %v", err)
	}

	if len(mock.flushCalls) != 1 {
		t.Errorf("WriteCandles() expected 1 flush call, got %d", len(mock.flushCalls))
	}

	expectedSQL := "INSERT INTO candles_1m VALUES"
	if len(mock.flushCalls) > 0 && mock.flushCalls[0] != expectedSQL {
		t.Errorf("WriteCandles() flush SQL = %v, want %v", mock.flushCalls[0], expectedSQL)
	}
}

func TestWriteCandles_NoFlushUnderBatchSize(t *testing.T) {
	cfg := Config{
		DSN:          "clickhouse://localhost:9000/test",
		Database:     "trades_db",
		TradesTable:  "trades",
		CandlesTable: "candles_1m",
		BatchSize:    5,
		MaxRetries:   3,
	}

	mock := newMockWriter(cfg)
	ctx := context.Background()

	// Test with fewer than batch size candles
	candles := []Candle{
		{Timestamp: time.Now(), PoolID: "pool1", Open: 1.0, High: 2.0, Low: 0.5, Close: 1.5, Volume: 100.0},
		{Timestamp: time.Now(), PoolID: "pool1", Open: 1.5, High: 2.5, Low: 1.0, Close: 2.0, Volume: 150.0},
	}

	err := mock.WriteCandles(ctx, candles)
	if err != nil {
		t.Errorf("WriteCandles() unexpected error = %v", err)
	}

	if len(mock.flushCalls) != 0 {
		t.Errorf("WriteCandles() expected 0 flush calls, got %d", len(mock.flushCalls))
	}

	if mock.candlesBatch.count != 2 {
		t.Errorf("WriteCandles() expected 2 items in batch, got %d", mock.candlesBatch.count)
	}
}
