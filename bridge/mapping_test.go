package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSubjectMappings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mapping.yaml")
	content := `mappings:
  - source: "dex.sol.swap."
    target: "legacy.swap."
  - source: "dex.sol.debug"
    drop: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write mapping file: %v", err)
	}

	mappings, err := LoadSubjectMappings(path)
	if err != nil {
		t.Fatalf("LoadSubjectMappings() error = %v", err)
	}
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].Target != "legacy.swap." {
		t.Fatalf("unexpected target %q", mappings[0].Target)
	}
	if !mappings[1].Drop {
		t.Fatalf("expected drop mapping")
	}
}

func TestMapperFromMappings(t *testing.T) {
	mapper, err := mapperFromMappings([]SubjectMapping{
		{Source: "dex.sol.swap.", Target: "legacy.swap."},
		{Source: "dex.sol.swap.raydium", Target: "legacy.swap.raydium"},
		{Source: "dex.sol.debug", Drop: true},
	})
	if err != nil {
		t.Fatalf("mapperFromMappings() error = %v", err)
	}

	subj, ok := mapper("dex.sol.swap.raydium.pool1")
	if !ok {
		t.Fatalf("expected mapping to succeed")
	}
	if subj != "legacy.swap.raydium.pool1" {
		t.Fatalf("unexpected mapped subject %q", subj)
	}

	subj, ok = mapper("dex.sol.swap.orca")
	if !ok {
		t.Fatalf("expected mapping to succeed for generic prefix")
	}
	if subj != "legacy.swap.orca" {
		t.Fatalf("unexpected mapped subject %q", subj)
	}

	if _, ok := mapper("dex.sol.debug"); ok {
		t.Fatalf("expected drop mapping to return ok=false")
	}

	subj, ok = mapper("dex.sol.other")
	if !ok || subj != "dex.sol.other" {
		t.Fatalf("expected identity mapping, got %q ok=%t", subj, ok)
	}
}
