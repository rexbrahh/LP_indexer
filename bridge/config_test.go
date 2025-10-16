package bridge

import "testing"

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		SourceURL:    "nats://source:4222",
		TargetURL:    "nats://target:4222",
		SourceStream: "DEX",
		TargetStream: "legacy",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestConfigMissing(t *testing.T) {
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(envSourceURL, "nats://source:4222")
	t.Setenv(envTargetURL, "nats://target:4222")
	t.Setenv(envSourceStream, "DEX")
	t.Setenv(envTargetStream, "legacy")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.SourceURL != "nats://source:4222" || cfg.TargetURL != "nats://target:4222" {
		t.Fatalf("unexpected URLs: %s -> %s", cfg.SourceURL, cfg.TargetURL)
	}
}
