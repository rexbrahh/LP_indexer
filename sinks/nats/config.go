package natsx

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultPublishTimeout = 5 * time.Second

	envNATSURL         = "NATS_URL"
	envNATSStream      = "NATS_STREAM"
	envNATSSubjectRoot = "NATS_SUBJECT_ROOT"
	envPublishTimeout  = "NATS_PUBLISH_TIMEOUT_MS"
)

// Config captures the runtime parameters for the JetStream publisher.
type Config struct {
	URL            string
	Stream         string
	SubjectRoot    string
	PublishTimeout time.Duration
}

// DefaultConfig initialises Config with defaults for optional fields.
func DefaultConfig() Config {
	return Config{
		SubjectRoot:    "dex.sol",
		PublishTimeout: defaultPublishTimeout,
	}
}

// Validate ensures required fields are populated and durations are sane.
func (c Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("NATS URL is required")
	}
	if c.Stream == "" {
		return fmt.Errorf("NATS stream is required")
	}
	if c.SubjectRoot == "" {
		return fmt.Errorf("subject root cannot be empty")
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("publish timeout must be positive")
	}
	return nil
}

// FromEnv constructs a Config from environment variables.
func FromEnv() (Config, error) {
	cfg := DefaultConfig()
	if v := os.Getenv(envNATSURL); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv(envNATSStream); v != "" {
		cfg.Stream = v
	}
	if v := os.Getenv(envNATSSubjectRoot); v != "" {
		cfg.SubjectRoot = v
	}
	if v := os.Getenv(envPublishTimeout); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", envPublishTimeout, err)
		}
		cfg.PublishTimeout = time.Duration(ms) * time.Millisecond
	}
	return cfg, cfg.Validate()
}
