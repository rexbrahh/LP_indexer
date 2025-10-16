package bridge

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SubjectMapping describes how to translate canonical subjects to legacy ones.
// When Drop is true the message is acknowledged but not forwarded downstream.
type SubjectMapping struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Drop   bool   `yaml:"drop"`
}

type mappingFile struct {
	Mappings []SubjectMapping `yaml:"mappings"`
}

// LoadSubjectMappings parses a YAML file containing subject translation rules.
func LoadSubjectMappings(path string) ([]SubjectMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read subject map: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("subject map %q is empty", path)
	}

	var payload mappingFile
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse subject map: %w", err)
	}
	if len(payload.Mappings) == 0 {
		return nil, fmt.Errorf("subject map contains no mappings")
	}

	cleaned := make([]SubjectMapping, 0, len(payload.Mappings))
	for i, m := range payload.Mappings {
		src := strings.TrimSpace(m.Source)
		tgt := strings.TrimSpace(m.Target)
		if src == "" {
			return nil, fmt.Errorf("mapping %d: source is required", i)
		}
		if !m.Drop && tgt == "" {
			return nil, fmt.Errorf("mapping %d: target is required when drop=false", i)
		}
		cleaned = append(cleaned, SubjectMapping{Source: src, Target: tgt, Drop: m.Drop})
	}
	return cleaned, nil
}

// mapperFromMappings constructs a prefix-based SubjectMapper from mapping rules.
func mapperFromMappings(mappings []SubjectMapping) (SubjectMapper, error) {
	if len(mappings) == 0 {
		return defaultSubjectMapper, nil
	}

	compiled := make([]compiledMapping, 0, len(mappings))
	for _, m := range mappings {
		compiled = append(compiled, compiledMapping{
			source: m.Source,
			target: m.Target,
			drop:   m.Drop,
		})
	}

	sort.Slice(compiled, func(i, j int) bool {
		if len(compiled[i].source) == len(compiled[j].source) {
			return compiled[i].source < compiled[j].source
		}
		return len(compiled[i].source) > len(compiled[j].source)
	})

	return func(subject string) (string, bool) {
		for _, rule := range compiled {
			if strings.HasPrefix(subject, rule.source) {
				if rule.drop {
					return "", false
				}
				return rule.target + subject[len(rule.source):], true
			}
		}
		return subject, true
	}, nil
}

type compiledMapping struct {
	source string
	target string
	drop   bool
}

// loadSubjectMappingsFS is a helper for tests to load from an fs.FS.
func loadSubjectMappingsFS(fsys fs.FS, path string) ([]SubjectMapping, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	var payload mappingFile
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload.Mappings, nil
}
