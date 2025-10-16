package meteora

import "errors"

// ErrNotImplemented signals that the actual Meteora decoding logic is pending.
var ErrNotImplemented = errors.New("meteora decoder not yet implemented")

// DecodeSwapEvent accepts raw instruction data and its contextual metadata and
// returns a normalised SwapEvent. The real implementation will parse the
// Meteora instruction layouts (DLMM / CPMM) and populate the struct.
func DecodeSwapEvent(data []byte, ctx *InstructionContext) (*SwapEvent, error) {
	_ = data
	_ = ctx
	return nil, ErrNotImplemented
}
