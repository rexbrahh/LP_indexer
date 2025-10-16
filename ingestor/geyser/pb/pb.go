package pb

import (
	"context"
	"fmt"
)

type CommitmentLevel int32

const (
	CommitmentLevel_PROCESSED CommitmentLevel = 0
	CommitmentLevel_CONFIRMED CommitmentLevel = 1
	CommitmentLevel_FINALIZED CommitmentLevel = 2
)

type GeyserClient interface {
	Subscribe(ctx context.Context, opts ...any) (Geyser_SubscribeClient, error)
}

type Geyser_SubscribeClient interface {
	Send(*SubscribeRequest) error
	Recv() (*SubscribeUpdate, error)
}

type SubscribeRequest struct {
	Slots              map[string]*SubscribeRequestFilterSlots
	Accounts           map[string]*SubscribeRequestFilterAccounts
	Transactions       map[string]*SubscribeRequestFilterTransactions
	TransactionsStatus map[string]*SubscribeRequestFilterTransactions
	Entry              map[string]*SubscribeRequestFilterEntry
	Blocks             map[string]*SubscribeRequestFilterBlocks
	BlocksMeta         map[string]*SubscribeRequestFilterBlocksMeta
	AccountsDataSlice  []*SubscribeRequestAccountsDataSlice
	Commitment         CommitmentLevel
}

type SubscribeRequestFilterSlots struct{}

type SubscribeRequestFilterAccounts struct {
	Account []string
	Owner   []string
	Filters []*SubscribeRequestFilterAccountsFilter
}

type SubscribeRequestFilterAccountsFilter struct{}

type SubscribeRequestFilterTransactions struct{}

type SubscribeRequestFilterEntry struct{}

type SubscribeRequestFilterBlocks struct{}

type SubscribeRequestFilterBlocksMeta struct{}

type SubscribeRequestAccountsDataSlice struct{}

type SubscribeUpdate struct {
	UpdateOneof any
}

type SubscribeUpdate_Slot struct {
	Slot *SlotUpdate
}

type SubscribeUpdate_Account struct {
	Account *AccountUpdate
}

type SubscribeUpdate_Transaction struct {
	Transaction *TransactionUpdate
}

type SubscribeUpdate_Block struct {
	Block *BlockUpdate
}

type SubscribeUpdate_BlockMeta struct {
	BlockMeta *BlockMetaUpdate
}

type SlotUpdate struct {
	Slot uint64
}

type AccountUpdate struct {
	Slot uint64
}

type TransactionUpdate struct {
	Slot uint64
}

type BlockUpdate struct {
	Slot uint64
}

type BlockMetaUpdate struct {
	Slot uint64
}

func NewGeyserClient(_ any) GeyserClient {
	return &noopClient{}
}

type noopClient struct{}

func (n *noopClient) Subscribe(context.Context, ...any) (Geyser_SubscribeClient, error) {
	return nil, fmt.Errorf("geyser client stub: not implemented")
}
