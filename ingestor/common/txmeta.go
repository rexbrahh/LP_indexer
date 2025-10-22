package common

import (
	"github.com/mr-tron/base58/base58"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// ConvertTxMeta builds a canonical dex.sol.v1.TxMeta from a Yellowstone
// SubscribeUpdateTransaction. Returns nil when the transaction or metadata is
// missing.
func ConvertTxMeta(tx *pb.SubscribeUpdateTransaction) *dexv1.TxMeta {
	if tx == nil {
		return nil
	}
	info := tx.GetTransaction()
	if info == nil {
		return nil
	}
	meta := info.GetMeta()
	if meta == nil {
		return nil
	}

	signature := base58.Encode(info.GetSignature())
	msg := &dexv1.TxMeta{
		ChainId: 501,
		Slot:    tx.GetSlot(),
		Sig:     signature,
		Success: meta.GetErr() == nil,
		CuUsed:  meta.GetComputeUnitsConsumed(),
		CuPrice: 0,
		LogMsgs: meta.GetLogMessages(),
	}
	return msg
}
