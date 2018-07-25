package db

import (
	"github.com/tendermint/go-amino"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/cosmos/ethermint/types"
)

// Maps address to account
type AccountMapper struct {
	accountKey sdk.StoreKey

	cdc *amino.Codec
}

func NewAccountMapper(accKey sdk.StoreKey, cdc *amino.Codec) AccountMapper {
	return AccountMapper{
		accountKey: accKey,
		cdc: cdc,
	}
}

func (am AccountMapper) GetAccount(ctx sdk.Context, addr common.Address) types.Account {
	store := ctx.KVStore(am.accountKey)
	bz := store.Get(addr.Bytes())
	if bz == nil {
		return types.Account{}
	}
	acc := types.Account{}
	err := am.cdc.UnmarshalBinary(bz, &acc)
	if err != nil {
		panic(err)
	}
	return acc
}

func (am AccountMapper) SetAccount(ctx sdk.Context, acc types.Account) {
	addr := acc.Address.Bytes()
	store := ctx.KVStore(am.accountKey)
	val, err := am.cdc.MarshalBinary(acc)
	if err != nil {
		panic(err)
	}
	store.Set(addr, val)
}

func (am AccountMapper) NewAccount(ctx sdk.Context, addr common.Address) sdk.Error {
	store := ctx.KVStore(am.accountKey)
	if bz := store.Get(addr.Bytes()); bz != nil {
		return sdk.ErrUnauthorized("Account for this address already exists")
	}
	acc := types.Account{
		Address: addr,
		AccountNonce: 0,
	}
	val, err := am.cdc.MarshalBinary(acc)
	if err != nil {
		return sdk.ErrInternal("Account encoding failed")
	}
	store.Set(addr.Bytes(), val)
	return nil
}

func (am AccountMapper) GetSequence(ctx sdk.Context, addr common.Address) (int64, sdk.Error) {
	store := ctx.KVStore(am.accountKey)
	bz := store.Get(addr.Bytes())
	if bz != nil {
		return 0, sdk.ErrInvalidAddress("Account for this address not in state")
	}
	acc := types.Account{}
	err := am.cdc.UnmarshalBinary(bz, acc)
	if err != nil {
		return 0, sdk.ErrInternal("Account decoding failed")
	}
	return acc.AccountNonce, nil
}