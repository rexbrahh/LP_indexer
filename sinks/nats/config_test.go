package natsx

import (
	"testing"
	"time"
)

func TestDefaultConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.URL = "nats://localhost:4222"
	cfg.Stream = "DEX"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestConfigValidateMissing(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing URL/stream")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(envNATSURL, "nats://user:pass@nats:4222")
	t.Setenv(envNATSStream, "DEX")
	t.Setenv(envNATSSubjectRoot, "dex.sol")
	t.Setenv(envPublishTimeout, "1500")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.URL != "nats://user:pass@nats:4222" {
		t.Fatalf("unexpected URL %s", cfg.URL)
	}
	if cfg.Stream != "DEX" {
		t.Fatalf("unexpected stream %s", cfg.Stream)
	}
	if cfg.SubjectRoot != "dex.sol" {
		t.Fatalf("unexpected subject root %s", cfg.SubjectRoot)
	}
	if cfg.PublishTimeout != 1500*time.Millisecond {
		t.Fatalf("unexpected timeout %s", cfg.PublishTimeout)
	}
}
