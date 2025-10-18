package parquet

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultFlushInterval = 15 * time.Minute
	defaultPrefix        = "dex/"

	envEndpoint       = "S3_ENDPOINT"
	envBucket         = "S3_BUCKET"
	envAccessKey      = "S3_ACCESS_KEY"
	envSecretKey      = "S3_SECRET_KEY"
	envFlushIntervalS = "PARQUET_FLUSH_INTERVAL_S"
	envPrefix         = "PARQUET_PREFIX"
)

// Config holds parameters for the Parquet writer.
type Config struct {
	Endpoint      string
	Bucket        string
	AccessKey     string
	SecretKey     string
	Prefix        string
	FlushInterval time.Duration
	BatchRows     int
	Region        string
}

// DefaultConfig sets optional fields to sensible defaults.
func DefaultConfig() Config {
	return Config{
		Prefix:        defaultPrefix,
		FlushInterval: defaultFlushInterval,
		BatchRows:     5000,
		Region:        "us-east-1",
	}
}

// Validate ensures mandatory fields are present.
func (c Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("S3 endpoint is required")
	}
	if c.Bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("S3 access key is required")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("S3 secret key is required")
	}
	if c.FlushInterval <= 0 {
		return fmt.Errorf("flush interval must be positive")
	}
	if c.Prefix == "" {
		return fmt.Errorf("object prefix cannot be empty")
	}
	if c.BatchRows <= 0 {
		return fmt.Errorf("batch rows must be positive")
	}
	if c.Region == "" {
		return fmt.Errorf("region must be set")
	}
	return nil
}

// FromEnv builds Config from environment variables.
func FromEnv() (Config, error) {
	cfg := DefaultConfig()
	if v := os.Getenv(envEndpoint); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv(envBucket); v != "" {
		cfg.Bucket = v
	}
	if v := os.Getenv(envAccessKey); v != "" {
		cfg.AccessKey = v
	}
	if v := os.Getenv(envSecretKey); v != "" {
		cfg.SecretKey = v
	}
	if v := os.Getenv(envPrefix); v != "" {
		cfg.Prefix = v
	}
	if v := os.Getenv(envFlushIntervalS); v != "" {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envFlushIntervalS, err)
		}
		cfg.FlushInterval = time.Duration(seconds) * time.Second
	}
	if v := os.Getenv("PARQUET_BATCH_ROWS"); v != "" {
		rows, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PARQUET_BATCH_ROWS: %w", err)
		}
		cfg.BatchRows = rows
	}
	if v := os.Getenv("PARQUET_REGION"); v != "" {
		cfg.Region = v
	}
	return cfg, cfg.Validate()
}
