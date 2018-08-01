package types

import (
	ethcmn "github.com/ethereum/go-ethereum/common"
)

// Account implements an Ethermint account.
//
// TODO: Add other fields to allow Account to store Ether, ERC20, ERC721, etc.
type Account struct {
	Address ethcmn.Address
	Nonce   int64
}

// NewAccount returns a new Account with a given address.
func NewAccount(addr ethcmn.Address) Account {
	return Account{
		Address: addr,
	}
}
