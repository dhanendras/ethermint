package types

import (
	"github.com/ethereum/go-ethereum/common"
)

// Account struct with address and AccountNonce
// TODO: Add other fields to allow Account to store Ether, ERC20, ERC721, etc.
type Account struct {
	Address      common.Address
	AccountNonce int64
}

func NewAccount(addr common.Address) Account {
	return Account{
		Address: addr,
	}
}
