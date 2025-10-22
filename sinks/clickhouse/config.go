package clickhouse

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	envSinkNATSURL         = "CH_SINK_NATS_URL"
	envSinkStream          = "CH_SINK_NATS_STREAM"
	envSinkSubjectRoot     = "CH_SINK_SUBJECT_ROOT"
	envSinkConsumer        = "CH_SINK_CONSUMER"
	envSinkPullBatch       = "CH_SINK_PULL_BATCH"
	envSinkPullTimeoutMS   = "CH_SINK_PULL_TIMEOUT_MS"
	envSinkFlushIntervalMS = "CH_SINK_FLUSH_INTERVAL_MS"

	envSinkDSN             = "CH_SINK_DSN"
	envSinkDatabase        = "CH_SINK_DATABASE"
	envSinkTradesTable     = "CH_SINK_TRADES_TABLE"
	envSinkCandlesTable    = "CH_SINK_CANDLES_TABLE"
	envSinkBatchSize       = "CH_SINK_BATCH_SIZE"
	envSinkMaxRetries      = "CH_SINK_MAX_RETRIES"
	envSinkRetryBackoffMS  = "CH_SINK_RETRY_BACKOFF_MS"
	envSinkRetryBackoffMax = "CH_SINK_RETRY_BACKOFF_MAX_MS"
)

// ServiceConfig drives the JetStream → ClickHouse sink service.
type ServiceConfig struct {
	NATSURL     string
	Stream      string
	SubjectRoot string
	Consumer    string
	PullBatch   int
	PullTimeout time.Duration
	Writer      Config
}

// Validate ensures required fields are populated.
func (c ServiceConfig) Validate() error {
	if c.NATSURL == "" {
		return fmt.Errorf("nats url is required")
	}
	if c.Stream == "" {
		return fmt.Errorf("nats stream is required")
	}
	if c.SubjectRoot == "" {
		return fmt.Errorf("subject root is required")
	}
	if c.Consumer == "" {
		return fmt.Errorf("consumer name is required")
	}
	if c.PullBatch <= 0 {
		return fmt.Errorf("pull batch must be positive")
	}
	if c.PullTimeout <= 0 {
		return fmt.Errorf("pull timeout must be positive")
	}
	return validateConfig(c.Writer)
}

// ServiceConfigFromEnv loads ServiceConfig from environment variables.
func ServiceConfigFromEnv() (ServiceConfig, error) {
	cfg := ServiceConfig{
		NATSURL:     os.Getenv(envSinkNATSURL),
		Stream:      os.Getenv(envSinkStream),
		SubjectRoot: valueOrDefault(os.Getenv(envSinkSubjectRoot), "dex.sol"),
		Consumer:    valueOrDefault(os.Getenv(envSinkConsumer), "clickhouse-sink"),
		PullBatch:   256,
		PullTimeout: 500 * time.Millisecond,
		Writer: Config{
			BatchSize:        512,
			FlushInterval:    1 * time.Second,
			MaxRetries:       3,
			RetryBackoffBase: 200 * time.Millisecond,
			RetryBackoffMax:  5 * time.Second,
		},
	}

	if v := os.Getenv(envSinkPullBatch); v != "" {
		if batch, err := strconv.Atoi(v); err == nil && batch > 0 {
			cfg.PullBatch = batch
		} else {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkPullBatch, v)
		}
	}
	if v := os.Getenv(envSinkPullTimeoutMS); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms <= 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkPullTimeoutMS, v)
		}
		cfg.PullTimeout = time.Duration(ms) * time.Millisecond
	}
	if v := os.Getenv(envSinkFlushIntervalMS); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms <= 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkFlushIntervalMS, v)
		}
		cfg.Writer.FlushInterval = time.Duration(ms) * time.Millisecond
	}

	if v := os.Getenv(envSinkDSN); v != "" {
		cfg.Writer.DSN = v
	}
	if v := os.Getenv(envSinkDatabase); v != "" {
		cfg.Writer.Database = v
	}
	if v := os.Getenv(envSinkTradesTable); v != "" {
		cfg.Writer.TradesTable = v
	}
	if v := os.Getenv(envSinkCandlesTable); v != "" {
		cfg.Writer.CandlesTable = v
	}
	if v := os.Getenv(envSinkBatchSize); v != "" {
		batch, err := strconv.Atoi(v)
		if err != nil || batch <= 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkBatchSize, v)
		}
		cfg.Writer.BatchSize = batch
	}
	if v := os.Getenv(envSinkMaxRetries); v != "" {
		retries, err := strconv.Atoi(v)
		if err != nil || retries < 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkMaxRetries, v)
		}
		cfg.Writer.MaxRetries = retries
	}
	if v := os.Getenv(envSinkRetryBackoffMS); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms < 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkRetryBackoffMS, v)
		}
		cfg.Writer.RetryBackoffBase = time.Duration(ms) * time.Millisecond
	}
	if v := os.Getenv(envSinkRetryBackoffMax); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms < 0 {
			return ServiceConfig{}, fmt.Errorf("invalid %s: %q", envSinkRetryBackoffMax, v)
		}
		cfg.Writer.RetryBackoffMax = time.Duration(ms) * time.Millisecond
	}

	return cfg, cfg.Validate()
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
