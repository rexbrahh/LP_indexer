package meteora

import "testing"

func TestSwapEventToProto(t *testing.T) {
	event := &SwapEvent{
		Signature: "sig",
		Slot:      123,
		MintBase:  "BASE",
		MintQuote: "QUOTE",
		DecBase:   6,
		DecQuote:  6,
		FeeBps:    30,
		Pool:      "pool",
	}

	msg := event.ToProto()
	if msg == nil {
		t.Fatal("expected proto message, got nil")
	}
	if msg.GetChainId() != solanaChainID {
		t.Fatalf("unexpected chain id %d", msg.GetChainId())
	}
	if msg.GetPoolId() != "pool" {
		t.Fatalf("unexpected pool id %s", msg.GetPoolId())
	}
	if msg.GetMintBase() != "BASE" || msg.GetMintQuote() != "QUOTE" {
		t.Fatalf("unexpected mints %s/%s", msg.GetMintBase(), msg.GetMintQuote())
	}
	if msg.GetFeeBps() != 30 {
		t.Fatalf("unexpected fee %d", msg.GetFeeBps())
	}
}
