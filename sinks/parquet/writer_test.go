package parquet

import "testing"

func TestWriterValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Endpoint = ""
	cfg.Bucket = "bucket"
	cfg.AccessKey = "access"
	cfg.SecretKey = "secret"

	w, err := NewWriter(cfg)
	if err != ErrWriterDisabled || w != nil {
		t.Fatalf("expected ErrWriterDisabled, got %v", err)
	}
}
