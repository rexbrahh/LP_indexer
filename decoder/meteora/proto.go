package meteora

import dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"

// ToProto projects the swap event into the canonical protobuf message used by
// downstream publishers. Fields that are not yet populated by the decoder are
// left at their zero values and will be filled in once decoding is implemented.
func (e *SwapEvent) ToProto() *dexv1.SwapEvent {
	if e == nil {
		return nil
	}

	programID := e.ProgramID
	if programID == "" {
		programID = "cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG"
	}

	msg := &dexv1.SwapEvent{
		ChainId: 501,
		Slot:    e.Slot,
		Sig:     e.Signature,

		ProgramId: programID,
		PoolId:    e.Pool,

		MintBase:    e.MintBase,
		MintQuote:   e.MintQuote,
		DecBase:     e.DecBase,
		DecQuote:    e.DecQuote,
		FeeBps:      e.FeeBps,
		Provisional: true,
	}

	if e.BaseDecreased {
		msg.BaseOut = e.BaseAmount
		msg.QuoteIn = e.QuoteAmount
	} else {
		msg.BaseIn = e.BaseAmount
		msg.QuoteOut = e.QuoteAmount
	}

	return msg
}
