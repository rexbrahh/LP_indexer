package helius

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultTimeout       = 10 * time.Second
	defaultBackoff       = 5 * time.Second
	defaultReplayWindow  = 64
	envHeliusGRPC        = "HELIUS_GRPC"
	envHeliusWS          = "HELIUS_WS"
	envHeliusAPIKey      = "HELIUS_API_KEY"
	envHeliusTimeoutMS   = "HELIUS_TIMEOUT_MS"
	envHeliusBackoffMS   = "HELIUS_BACKOFF_MS"
	envHeliusReplaySlots = "HELIUS_REPLAY_SLOTS"
)

// Config captures the runtime parameters required to connect to the Helius
// LaserStream and WebSocket endpoints.
type Config struct {
	GRPCEndpoint string
	WSEndpoint   string
	APIKey       string

	RequestTimeout   time.Duration
	ReconnectBackoff time.Duration
	ReplaySlots      uint64
	ProgramFilters   map[string]string
}

// DefaultConfig returns a Config populated with sensible defaults. Endpoints
// and API key remain empty because they are environment-specific.
func DefaultConfig() *Config {
	return &Config{
		RequestTimeout:   defaultTimeout,
		ReconnectBackoff: defaultBackoff,
		ReplaySlots:      defaultReplayWindow,
	}
}

// Validate ensures critical fields are present and durations fall within sane
// bounds.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	var missing []string
	if c.GRPCEndpoint == "" {
		missing = append(missing, "GRPCEndpoint")
	}
	if c.WSEndpoint == "" {
		missing = append(missing, "WSEndpoint")
	}
	if c.APIKey == "" {
		missing = append(missing, "APIKey")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required Helius config fields: %v", missing)
	}

	if c.RequestTimeout <= 0 {
		return fmt.Errorf("invalid RequestTimeout: %s", c.RequestTimeout)
	}
	if c.ReconnectBackoff <= 0 {
		return fmt.Errorf("invalid ReconnectBackoff: %s", c.ReconnectBackoff)
	}
	if c.ReplaySlots == 0 {
		return errors.New("ReplaySlots must be >= 1")
	}
	return nil
}

// FromEnv builds a Config from the canonical environment variables and applies
// defaults for optional settings.
func FromEnv() (*Config, error) {
	cfg := DefaultConfig()
	cfg.GRPCEndpoint = os.Getenv(envHeliusGRPC)
	cfg.WSEndpoint = os.Getenv(envHeliusWS)
	cfg.APIKey = os.Getenv(envHeliusAPIKey)

	if v := os.Getenv(envHeliusTimeoutMS); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			cfg.RequestTimeout = time.Duration(ms) * time.Millisecond
		} else if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", envHeliusTimeoutMS, err)
		}
	}

	if v := os.Getenv(envHeliusBackoffMS); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			cfg.ReconnectBackoff = time.Duration(ms) * time.Millisecond
		} else if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", envHeliusBackoffMS, err)
		}
	}

	if v := os.Getenv(envHeliusReplaySlots); v != "" {
		if slots, err := strconv.Atoi(v); err == nil && slots > 0 {
			cfg.ReplaySlots = uint64(slots)
		} else if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", envHeliusReplaySlots, err)
		}
	}

	return cfg, cfg.Validate()
}
