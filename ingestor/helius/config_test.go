package helius

import (
	"os"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		GRPCEndpoint:     "grpc.example.com:443",
		WSEndpoint:       "wss://example.com",
		APIKey:           "secret",
		RequestTimeout:   time.Second,
		ReconnectBackoff: time.Second,
		ReplaySlots:      64,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestConfigValidateMissing(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing fields")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(envHeliusGRPC, "grpc.helius.dev:443")
	t.Setenv(envHeliusWS, "wss://helius.dev/ws")
	t.Setenv(envHeliusAPIKey, "secret-key")
	t.Setenv(envHeliusTimeoutMS, "1500")
	t.Setenv(envHeliusBackoffMS, "5000")
	t.Setenv(envHeliusReplaySlots, "128")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}

	if cfg.RequestTimeout != 1500*time.Millisecond {
		t.Fatalf("RequestTimeout mismatch, got %s", cfg.RequestTimeout)
	}
	if cfg.ReconnectBackoff != 5*time.Second {
		t.Fatalf("ReconnectBackoff mismatch, got %s", cfg.ReconnectBackoff)
	}
	if cfg.ReplaySlots != 128 {
		t.Fatalf("ReplaySlots mismatch, got %d", cfg.ReplaySlots)
	}
}

func TestFromEnvInvalidDurations(t *testing.T) {
	t.Setenv(envHeliusGRPC, "grpc.helius.dev:443")
	t.Setenv(envHeliusWS, "wss://helius.dev/ws")
	t.Setenv(envHeliusAPIKey, "secret-key")
	t.Setenv(envHeliusTimeoutMS, "invalid")

	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error for invalid timeout")
	}

	// Clean up invalid env for subsequent tests
	_ = os.Unsetenv(envHeliusTimeoutMS)
}
