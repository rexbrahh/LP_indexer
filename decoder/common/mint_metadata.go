package common

import (
	"fmt"
	"sync"
)

// MintMetadata represents metadata for a Solana token mint
type MintMetadata struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals"`
	Name     string `json:"name"`
}

// MintMetadataProvider is the interface that Engineer B will implement
// to provide mint metadata lookup functionality
type MintMetadataProvider interface {
	// GetMintMetadata retrieves metadata for a given mint address
	GetMintMetadata(mintAddress string) (*MintMetadata, error)

	// GetDecimals returns just the decimal places for a mint (convenience method)
	GetDecimals(mintAddress string) (uint8, error)

	// CacheMintMetadata pre-loads metadata for a batch of mints
	CacheMintMetadata(mintAddresses []string) error
}

// InMemoryMintMetadataProvider is a simple in-memory implementation
// This is a stub/mock for testing until Engineer B provides the real implementation
type InMemoryMintMetadataProvider struct {
	mu       sync.RWMutex
	metadata map[string]*MintMetadata
}

// NewInMemoryMintMetadataProvider creates a new in-memory provider with common Solana mints
func NewInMemoryMintMetadataProvider() *InMemoryMintMetadataProvider {
	provider := &InMemoryMintMetadataProvider{
		metadata: make(map[string]*MintMetadata),
	}

	// Pre-populate with common Solana tokens
	commonMints := []*MintMetadata{
		{
			Address:  "So11111111111111111111111111111111111111112",
			Symbol:   "SOL",
			Decimals: 9,
			Name:     "Wrapped SOL",
		},
		{
			Address:  "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			Symbol:   "USDC",
			Decimals: 6,
			Name:     "USD Coin",
		},
		{
			Address:  "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			Symbol:   "USDT",
			Decimals: 6,
			Name:     "USDT",
		},
		{
			Address:  "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs",
			Symbol:   "ORCA",
			Decimals: 6,
			Name:     "Orca",
		},
	}

	for _, mint := range commonMints {
		provider.metadata[mint.Address] = mint
	}

	return provider
}

// GetMintMetadata retrieves metadata for a given mint address
func (p *InMemoryMintMetadataProvider) GetMintMetadata(mintAddress string) (*MintMetadata, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metadata, ok := p.metadata[mintAddress]
	if !ok {
		return nil, fmt.Errorf("mint metadata not found for address: %s", mintAddress)
	}

	return metadata, nil
}

// GetDecimals returns just the decimal places for a mint
func (p *InMemoryMintMetadataProvider) GetDecimals(mintAddress string) (uint8, error) {
	metadata, err := p.GetMintMetadata(mintAddress)
	if err != nil {
		return 0, err
	}

	return metadata.Decimals, nil
}

// CacheMintMetadata pre-loads metadata for a batch of mints
func (p *InMemoryMintMetadataProvider) CacheMintMetadata(mintAddresses []string) error {
	// In the stub implementation, this is a no-op since we pre-populate common mints
	// Engineer B's implementation will fetch from an external source
	return nil
}

// AddMintMetadata adds or updates metadata for a mint (for testing)
func (p *InMemoryMintMetadataProvider) AddMintMetadata(metadata *MintMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metadata[metadata.Address] = metadata
}

// CanonicalOrdering defines the priority for base asset ordering
var CanonicalOrdering = []string{
	"USDC",
	"USDT",
	"SOL",
}

// DetermineBaseQuote determines which mint should be the base and which should be quote
// based on canonical ordering rules (USDC > USDT > SOL > others)
func DetermineBaseQuote(mintA, mintB string, provider MintMetadataProvider) (base, quote string, err error) {
	metadataA, err := provider.GetMintMetadata(mintA)
	if err != nil {
		return "", "", err
	}

	metadataB, err := provider.GetMintMetadata(mintB)
	if err != nil {
		return "", "", err
	}

	// Find priority for each symbol
	priorityA := getPriority(metadataA.Symbol)
	priorityB := getPriority(metadataB.Symbol)

	// Lower priority number = higher priority (e.g., USDC=0 is highest)
	if priorityA < priorityB {
		return mintA, mintB, nil
	} else if priorityB < priorityA {
		return mintB, mintA, nil
	}

	// If same priority (or both not in canonical list), use lexicographic order
	if mintA < mintB {
		return mintA, mintB, nil
	}
	return mintB, mintA, nil
}

func getPriority(symbol string) int {
	for i, canonical := range CanonicalOrdering {
		if symbol == canonical {
			return i
		}
	}
	// Not in canonical list - assign lowest priority
	return len(CanonicalOrdering)
}
