package parquet

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Endpoint = "http://minio:9000"
	cfg.Bucket = "dex-parquet"
	cfg.AccessKey = "access"
	cfg.SecretKey = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestConfigMissing(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing endpoint/bucket/credentials")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(envEndpoint, "http://minio:9000")
	t.Setenv(envBucket, "dex-parquet")
	t.Setenv(envAccessKey, "access")
	t.Setenv(envSecretKey, "secret")
	t.Setenv(envPrefix, "archive/")
	t.Setenv(envFlushIntervalS, "600")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.Prefix != "archive/" {
		t.Fatalf("unexpected prefix %s", cfg.Prefix)
	}
	if cfg.FlushInterval != 10*time.Minute {
		t.Fatalf("unexpected flush interval %s", cfg.FlushInterval)
	}
}
