package bridge

import (
	"fmt"
	"os"
)

const (
	envSourceURL    = "BRIDGE_SOURCE_NATS_URL"
	envTargetURL    = "BRIDGE_TARGET_NATS_URL"
	envSourceStream = "BRIDGE_SOURCE_STREAM"
	envTargetStream = "BRIDGE_TARGET_STREAM"
)

// Config controls the source/target JetStream endpoints.
type Config struct {
	SourceURL    string
	TargetURL    string
	SourceStream string
	TargetStream string
}

// Validate ensures required fields are populated.
func (c Config) Validate() error {
	if c.SourceURL == "" {
		return fmt.Errorf("source NATS URL is required")
	}
	if c.TargetURL == "" {
		return fmt.Errorf("target NATS URL is required")
	}
	if c.SourceStream == "" {
		return fmt.Errorf("source stream is required")
	}
	if c.TargetStream == "" {
		return fmt.Errorf("target stream is required")
	}
	return nil
}

// FromEnv loads configuration from environment variables.
func FromEnv() (Config, error) {
	cfg := Config{
		SourceURL:    os.Getenv(envSourceURL),
		TargetURL:    os.Getenv(envTargetURL),
		SourceStream: os.Getenv(envSourceStream),
		TargetStream: os.Getenv(envTargetStream),
	}
	return cfg, cfg.Validate()
}
