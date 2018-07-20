package types

import (
	"math/big"
	"reflect"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	sdkAddress common.Address
)

// Copied Ethereum Transaction to implement both sdk.Msg and sdk.Tx interface
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

// Implement sdk.Msg Interface
func (tx Transaction) Type() string { return "Eth" }

// Implement sdk.Msg Interface
func (tx Transaction) ValidateBasic() sdk.Error {
	if tx.data.Price.Sign() != 1 {
		return ErrInvalidValue(2, "Price must be positive")
	}
	if tx.data.Amount.Sign() != 1 {
		return ErrInvalidValue(2, "Amount must be positive")
	}
	return nil
}

// Implement sdk.Msg Interface. Can't use this because different signer types sign over different bytes
func (tx Transaction) GetSignBytes() []byte {
	return nil
}

// Implement sdk.Msg Interface. This won't work until tx is already signed
func (tx Transaction) GetSigners() []sdk.AccAddress {
	addr := tx.from.Load().([]byte)
	if addr == nil {
		return nil
	}
	return []sdk.AccAddress{addr}
}

// Return inner txdata struct
func (tx Transaction) TxData() txdata {
	return tx.data
}

// Convert this sdk copy of Ethereum transaction to Ethereum transaction
func (tx Transaction) ConvertTx() types.Transaction {

	ethTx := types.NewTransaction(tx.data.AccountNonce, *tx.data.Recipient, tx.data.Amount,
		tx.data.GasLimit, tx.data.Price, tx.data.Payload)

	// Must somehow set ethTx.data.{V, R, S}
	// Currently the only idea I have is to make ConvertTx take in a signer
	// Depending on signer type, reconstruct sig []byte from V, R, S
	// Call ethTx.WithSignature(signer, sig)
	// Not ideal.

	return *ethTx
}

// Implement sdk.Tx interface
func (tx Transaction) GetMsgs() []sdk.Msg {
	if reflect.DeepEqual(*tx.data.Recipient, sdkAddress) {
		cdc := NewCodec()
		innerTx := InnerTransaction{}
		err := cdc.UnmarshalBinary(tx.data.Payload, &innerTx)
		if err != nil {
			return nil
		}
		return innerTx.GetMsgs()
	}
	return []sdk.Msg{tx}
}

// Get inner tx. Note: Will panic if decoding fails
func (tx Transaction) GetInnerTx() (InnerTransaction, sdk.Error) {
	cdc := NewCodec()
	innerTx := InnerTransaction{}
	err := cdc.UnmarshalBinary(tx.data.Payload, &innerTx)
	if err != nil {
		return InnerTransaction{}, sdk.ErrTxDecode("Inner transaction decoding failed")
	}
	return innerTx, nil
}

// Inner Transaction to be encoded into payload to handle sdk Msgs
type InnerTransaction struct {
	Msgs       []sdk.Msg
	Signatures [][]byte
}

// Implement sdk.Tx interface
func (tx InnerTransaction) GetMsgs() []sdk.Msg {
	return tx.Msgs
}

// Return all required signers of Tx accumulated from msgs
func (tx InnerTransaction) GetRequiredSigners() []common.Address {
	seen := map[string]bool{}
	var signers []common.Address
	for _, msg := range tx.GetMsgs() {
		for _, addr := range msg.GetSigners() {
			if !seen[addr.String()] {
				signers = append(signers, common.BytesToAddress(addr))
				seen[addr.String()] = true
			}
		}
	}
	return signers
}

// Sets SDKAddress. Only allowed to be set once
func SetSDKAddress(addr common.Address) {
	if sdkAddress.Bytes() == nil {
		sdkAddress = addr
	}
}

func NewCodec() *wire.Codec {
	cdc := wire.NewCodec()
	cdc.RegisterInterface((*sdk.Msg)(nil), nil)
	cdc.RegisterConcrete(InnerTransaction{}, "types/InnerTx", nil)
	return cdc
}
