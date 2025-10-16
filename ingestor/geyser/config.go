package geyser

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds Geyser client configuration
type Config struct {
	// Endpoint is the Geyser gRPC endpoint (e.g., "grpc.chainstack.com:443")
	Endpoint string `yaml:"endpoint"`

	// APIKey is the authentication key for the Geyser endpoint
	APIKey string `yaml:"api_key"`

	// ProgramFilters maps friendly names to Solana program IDs to filter
	ProgramFilters map[string]string `yaml:"program_filters"`
}

// LoadConfig loads configuration from environment variables and programs.yaml
func LoadConfig(programsYAMLPath string) (*Config, error) {
	cfg := &Config{
		Endpoint: os.Getenv("GEYSER_ENDPOINT"),
		APIKey:   os.Getenv("GEYSER_API_KEY"),
		ProgramFilters: make(map[string]string),
	}

	// Load program filters from YAML
	if programsYAMLPath != "" {
		filters, err := loadProgramFilters(programsYAMLPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load program filters: %w", err)
		}
		cfg.ProgramFilters = filters
	}

	return cfg, nil
}

// loadProgramFilters reads program IDs from a YAML file
func loadProgramFilters(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read programs file: %w", err)
	}

	var config struct {
		Programs map[string]string `yaml:"programs"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse programs YAML: %w", err)
	}

	return config.Programs, nil
}

// Validate checks that required configuration fields are set
func (c *Config) Validate() error {
	var errors []string

	if c.Endpoint == "" {
		errors = append(errors, "GEYSER_ENDPOINT is required")
	}

	if c.APIKey == "" {
		errors = append(errors, "GEYSER_API_KEY is required")
	}

	if len(c.ProgramFilters) == 0 {
		errors = append(errors, "at least one program filter is required")
	}

	// Validate program IDs are valid base58 strings (basic check)
	for name, programID := range c.ProgramFilters {
		if programID == "" {
			errors = append(errors, fmt.Sprintf("program filter '%s' has empty program ID", name))
		}
		// Basic length check for Solana addresses (32-44 characters in base58)
		if len(programID) < 32 || len(programID) > 44 {
			errors = append(errors, fmt.Sprintf("program filter '%s' has invalid program ID length: %s", name, programID))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// String returns a sanitized string representation of the config
func (c *Config) String() string {
	// Mask API key for logging
	maskedKey := c.APIKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "****" + maskedKey[len(maskedKey)-4:]
	} else if maskedKey != "" {
		maskedKey = "****"
	}

	var programs []string
	for name, id := range c.ProgramFilters {
		programs = append(programs, fmt.Sprintf("%s=%s", name, id))
	}

	return fmt.Sprintf("Config{Endpoint=%s, APIKey=%s, Programs=[%s]}",
		c.Endpoint, maskedKey, strings.Join(programs, ", "))
}
