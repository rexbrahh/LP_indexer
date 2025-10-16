package meteora

import (
	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

const solanaChainID = 501

// ToProto projects the swap event into the canonical protobuf message used by
// downstream publishers. Fields that are not yet populated by the decoder are
// left at their zero values and will be filled in once decoding is implemented.
func (e *SwapEvent) ToProto() *dexv1.SwapEvent {
	if e == nil {
		return nil
	}

	msg := &dexv1.SwapEvent{
		ChainId: solanaChainID,
		Slot:    e.Slot,
		Sig:     e.Signature,

		ProgramId: MeteoraProgramID,
		PoolId:    e.Pool,

		MintBase:    e.MintBase,
		MintQuote:   e.MintQuote,
		DecBase:     e.DecBase,
		DecQuote:    e.DecQuote,
		BaseIn:      0,
		BaseOut:     0,
		QuoteIn:     0,
		QuoteOut:    0,
		FeeBps:      e.FeeBps,
		Provisional: true,
	}

	// DLMM/CPMM reserve information will be populated once the decoder captures
	// those values from the instruction/log stream.
	return msg
}
