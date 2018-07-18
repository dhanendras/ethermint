package types

import (
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Ethereum Transaction will implement both sdk.Msg and sdk.Tx interface
type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	Price        *big.Int        `json:"gasPrice" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

func (tx Transaction) Type() string { return "Eth" }

func (tx Transaction) ValidateBasic() sdk.Error {
	if tx.data.Price.Sign() != 1 {
		return ErrInvalidValue(2, "Price must be positive")
	}
	if tx.data.Amount.Sign() != 1 {
		return ErrInvalidValue(2, "Amount must be positive")
	}
	return nil
}

// For now, use Hash value of FrontierSigner
func (tx Transaction) GetSignBytes() []byte {
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
	}).Bytes()
}

// This most likely does not work
func (tx Transaction) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{tx.from.Load().([]byte)}
}

func (tx Transaction) GetMsgs() []sdk.Msg {
	return []sdk.Msg{tx}
}

// Hashing taken from go-ethereum
func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}