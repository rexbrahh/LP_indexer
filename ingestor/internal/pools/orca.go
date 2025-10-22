package pools

import (
	"encoding/binary"
	"fmt"

	"github.com/mr-tron/base58/base58"
)

const (
	orcaDiscriminatorLen    = 8
	orcaConfigOffset        = orcaDiscriminatorLen
	orcaFeeRateOffset       = orcaConfigOffset + 32 + 1 + 2 + 2
	orcaProtocolFeeOffset   = orcaFeeRateOffset + 2
	orcaLiquidityOffset     = orcaProtocolFeeOffset + 2
	orcaSqrtPriceOffset     = orcaLiquidityOffset + 16
	orcaTickOffset          = orcaSqrtPriceOffset + 16
	orcaProtocolFeeAOOffset = orcaTickOffset + 4
	orcaProtocolFeeBOOffset = orcaProtocolFeeAOOffset + 8
	orcaTokenMintAOffset    = orcaProtocolFeeBOOffset + 8
	orcaTokenVaultAOffset   = orcaTokenMintAOffset + 32
	orcaFeeGrowthAOffset    = orcaTokenVaultAOffset + 32
	orcaTokenMintBOffset    = orcaFeeGrowthAOffset + 16
	orcaTokenVaultBOffset   = orcaTokenMintBOffset + 32
	orcaRequiredLength      = orcaTokenVaultBOffset + 32
)

// OrcaPoolInfo captures metadata required for swap decoding.
type OrcaPoolInfo struct {
	Config      string
	FeeRate     uint16 // stored as hundredths of a basis point
	ProtocolFee uint16
	TokenMintA  string
	TokenMintB  string
	TokenVaultA string
	TokenVaultB string
}

// DecodeOrcaPool extracts pool metadata from raw whirlpool account data.
func DecodeOrcaPool(data []byte) (*OrcaPoolInfo, error) {
	if len(data) < orcaRequiredLength {
		return nil, fmt.Errorf("orca pool account too short: have %d want >= %d", len(data), orcaRequiredLength)
	}
	info := &OrcaPoolInfo{
		Config:      base58.Encode(data[orcaConfigOffset : orcaConfigOffset+32]),
		FeeRate:     binary.LittleEndian.Uint16(data[orcaFeeRateOffset : orcaFeeRateOffset+2]),
		ProtocolFee: binary.LittleEndian.Uint16(data[orcaProtocolFeeOffset : orcaProtocolFeeOffset+2]),
		TokenMintA:  base58.Encode(data[orcaTokenMintAOffset : orcaTokenMintAOffset+32]),
		TokenVaultA: base58.Encode(data[orcaTokenVaultAOffset : orcaTokenVaultAOffset+32]),
		TokenMintB:  base58.Encode(data[orcaTokenMintBOffset : orcaTokenMintBOffset+32]),
		TokenVaultB: base58.Encode(data[orcaTokenVaultBOffset : orcaTokenVaultBOffset+32]),
	}
	return info, nil
}
