package orca_whirlpool

import dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"

const solanaChainID = 501

// ToProto materializes the canonical protobuf SwapEvent for downstream publishers.
// The method currently maps core identifiers and metadata; amount orientation and
// CLMM state fields will be populated once the canonical decoder wiring lands.
func (e *SwapEvent) ToProto() *dexv1.SwapEvent {
	if e == nil {
		return nil
	}

	event := &dexv1.SwapEvent{
		ChainId:     solanaChainID,
		Slot:        e.Slot,
		Sig:         e.Signature,
		ProgramId:   WhirlpoolProgramID,
		PoolId:      e.PoolAddress,
		MintBase:    e.BaseAsset,
		MintQuote:   e.QuoteAsset,
		DecBase:     0,
		DecQuote:    0,
		BaseIn:      0,
		BaseOut:     0,
		QuoteIn:     0,
		QuoteOut:    0,
		Provisional: true,
	}

	// TODO: wire in canonical amount orientation once pair normalization is available.

	// TODO: populate sqrt price, reserves, and fee basis points once the decoder exposes them.

	return event
}
