package pools

import (
	"encoding/binary"
	"fmt"
)

const (
	poolHeaderLen          = 8 // anchor discriminator
	poolConfigOffset       = poolHeaderLen + 1
	poolConfigEnd          = poolConfigOffset + 32
	ammHeaderLen           = 8
	ammTradeFeeOffset      = ammHeaderLen + 1 + 2 + 32 + 4
	ammRequiredLength      = ammTradeFeeOffset + 4
	ApproxConfigAccountMax = 256
)

var (
	ammConfigDiscriminator = [8]byte{218, 244, 33, 104, 203, 203, 43, 111}
	poolStateDiscriminator = [8]byte{247, 237, 227, 245, 215, 195, 222, 70}
)

func HasAmmConfigDiscriminator(data []byte) bool {
	return len(data) >= 8 && equalDiscriminator(data[:8], ammConfigDiscriminator[:])
}

func HasPoolDiscriminator(data []byte) bool {
	return len(data) >= 8 && equalDiscriminator(data[:8], poolStateDiscriminator[:])
}

func equalDiscriminator(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DecodeRaydiumPool extracts the amm_config pubkey from a Raydium CLMM pool account.
func DecodeRaydiumPool(data []byte) ([]byte, error) {
	if len(data) < poolConfigEnd {
		return nil, fmt.Errorf("raydium pool account too short: have %d want >= %d", len(data), poolConfigEnd)
	}
	return data[poolConfigOffset:poolConfigEnd], nil
}

// DecodeAmmConfig parses the Raydium `AmmConfig` account and returns the trade fee rate
// expressed in the on-chain denominator (1e-6 units).
func DecodeAmmConfig(data []byte) (uint32, error) {
	if len(data) < ammRequiredLength {
		return 0, fmt.Errorf("amm config account too short: have %d want >= %d", len(data), ammRequiredLength)
	}
	return binary.LittleEndian.Uint32(data[ammTradeFeeOffset : ammTradeFeeOffset+4]), nil
}
