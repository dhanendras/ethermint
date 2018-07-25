package types

import (
	"math/big"
	"reflect"
	"sync/atomic"
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/ethereum/go-ethereum/common"
)

var (
	sdkAddress common.Address
	Cdc = NewCodec()
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

// Implement sdk.Msg Interface
func (tx Transaction) Type() string { return "Eth" }

// Implement sdk.Msg Interface
func (tx Transaction) ValidateBasic() sdk.Error {
	if tx.data.Price.Sign() != 1 {
		return ErrInvalidValue(DefaultCodespace, "Price must be positive")
	}
	if tx.data.Amount.Sign() != 1 {
		return ErrInvalidValue(DefaultCodespace, "Amount must be positive")
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
		innerTx := EmbeddedTx{}
		err := cdc.UnmarshalBinary(tx.data.Payload, &innerTx)
		if err != nil {
			return nil
		}
		return innerTx.GetMsgs()
	}
	return []sdk.Msg{tx}
}

// Get inner tx. Note: Will panic if decoding fails
func (tx Transaction) GetEmbeddedTx() (EmbeddedTx, sdk.Error) {
	cdc := NewCodec()
	innerTx := EmbeddedTx{}
	err := cdc.UnmarshalBinary(tx.data.Payload, &innerTx)
	if err != nil {
		return EmbeddedTx{}, sdk.ErrTxDecode("Inner sdk transaction decoding failed")
	}
	return innerTx, nil
}

// EmbeddedTx to be encoded into payload to handle sdk Msgs
type EmbeddedTx struct {
	Msgs       []sdk.Msg
	Signatures [][]byte
}

// Implement sdk.Tx interface
func (tx EmbeddedTx) GetMsgs() []sdk.Msg {
	return tx.Msgs
}

// Return all required signers of Tx accumulated from msgs
func (tx EmbeddedTx) GetRequiredSigners() []common.Address {
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

// Creates simple SignDoc for EmbeddedTx signer to sign over
type EmbeddedSignDoc struct {
	ChainID  string
	Msgs     []json.RawMessage
	Sequence int64
}

// Creates signBytes for signer with given arguments
func EmbeddedSignBytes(chainID string, msgs []sdk.Msg, sequence int64) []byte {
	var msgsBytes []json.RawMessage
	for _, msg := range msgs {
		msgsBytes = append(msgsBytes, json.RawMessage(msg.GetSignBytes()))
	}
	signDoc := EmbeddedSignDoc{
		ChainID: chainID,
		Msgs: msgsBytes,
		Sequence: sequence,
	}
	bz, err := Cdc.MarshalJSON(signDoc)
	if err != nil {
		panic(err)
	}
	return bz
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
	cdc.RegisterConcrete(EmbeddedTx{}, "types/EmbeddedTx", nil)
	cdc.RegisterConcrete(Account{}, "types/Account", nil)
	return cdc
}
