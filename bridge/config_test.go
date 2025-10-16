package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

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
	t.Setenv(envMetricsAddr, ":9090")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.SourceURL != "nats://source:4222" || cfg.TargetURL != "nats://target:4222" {
		t.Fatalf("unexpected URLs: %s -> %s", cfg.SourceURL, cfg.TargetURL)
	}
	if cfg.MetricsAddr != ":9090" {
		t.Fatalf("expected metrics addr :9090, got %q", cfg.MetricsAddr)
	}
}

func TestFromEnvWithSubjectMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "map.yaml")
	if err := os.WriteFile(path, []byte("mappings:\n  - source: a\n    target: b\n"), 0o644); err != nil {
		t.Fatalf("write map file: %v", err)
	}

	t.Setenv(envSourceURL, "nats://source:4222")
	t.Setenv(envTargetURL, "nats://target:4222")
	t.Setenv(envSourceStream, "DEX")
	t.Setenv(envTargetStream, "legacy")
	t.Setenv(envSubjectMapPath, path)
	t.Setenv(envMetricsAddr, ":7070")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if len(cfg.SubjectMappings) != 1 {
		t.Fatalf("expected one mapping, got %d", len(cfg.SubjectMappings))
	}
	if cfg.SubjectMappings[0].Target != "b" {
		t.Fatalf("unexpected target %q", cfg.SubjectMappings[0].Target)
	}
	if cfg.MetricsAddr != ":7070" {
		t.Fatalf("unexpected metrics addr %q", cfg.MetricsAddr)
	}
}
