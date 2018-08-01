package types

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sync"
	"sync/atomic"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	sdkAddress     ethcmn.Address
	sdkAddressOnce sync.Once
	Cdc            = NewCodec()
)

type (
	// Transaction implements the Ethereum transaction structure as an exact
	// copy. It implements the Cosmos sdk.Tx interface. Due to the private
	// fields, it must be replicated here and cannot be embedded or used
	// directly.
	Transaction struct {
		data TxData

		// caches
		hash atomic.Value
		size atomic.Value
		from atomic.Value
	}

	// TxData implements the Ethereum transaction data structure as an exact
	// copy. It is used solely as intended in Ethereum abiding by the protocol
	// except for the payload field which may embed a Cosmos SDK transaction.
	TxData struct {
		AccountNonce uint64          `json:"nonce"    gencodec:"required"`
		Price        *big.Int        `json:"gasPrice" gencodec:"required"`
		GasLimit     uint64          `json:"gas"      gencodec:"required"`
		Recipient    *ethcmn.Address `json:"to"       rlp:"nil"` // nil means contract creation
		Amount       *big.Int        `json:"value"    gencodec:"required"`
		Payload      []byte          `json:"input"    gencodec:"required"`

		// signature values
		V *big.Int `json:"v" gencodec:"required"`
		R *big.Int `json:"r" gencodec:"required"`
		S *big.Int `json:"s" gencodec:"required"`

		// hash is only used when marshaling to JSON
		Hash *ethcmn.Hash `json:"hash" rlp:"-"`
	}
)

// TxData returns the Ethereum transaction data.
func (tx Transaction) TxData() TxData {
	return tx.data
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

// Implement sdk.Msg Interface. Can't use this because signBytes is hashed with chainID
func (tx Transaction) GetSignBytes() []byte {
	return nil
}

// GetSigners implements the Cosmos sdk.Msg interface.
//
// CONTRACT: The transaction must already be signed.
func (tx Transaction) GetSigners() []sdk.AccAddress {
	addr := tx.from.Load().([]byte)
	if addr == nil {
		return nil
	}

	return []sdk.AccAddress{addr}
}

// Convert this sdk copy of Ethereum transaction to Ethereum transaction
func (tx Transaction) ConvertTx(chainID *big.Int) ethtypes.Transaction {
	ethTx := ethtypes.NewTransaction(
		tx.data.AccountNonce, *tx.data.Recipient, tx.data.Amount,
		tx.data.GasLimit, tx.data.Price, tx.data.Payload,
	)

	// Must somehow set ethTx.data.{V, R, S}
	// Currently the only idea I have is to make ConvertTx take in a signer
	// Reconstruct sig []byte from V, R, S
	// Call ethTx.WithSignature(signer, sig)
	// Not ideal.
	sig := recoverSig(tx.data.V, tx.data.R, tx.data.S, chainID)
	signer := ethtypes.NewEIP155Signer(chainID)
	ethTx.WithSignature(signer, sig)

	return *ethTx
}

// IsSDKTx returns a boolean reflecting if the transaction is an SDK
// transaction or not based on the recipient address.
func (tx Transaction) IsEmbeddedTx() bool {
	return bytes.Equal(tx.data.Recipient.Bytes(), sdkAddress.Bytes())
}

// GetMsgs implements the Cosmos sdk.Tx interface. If the to/recipient address
// is the SDK address, the inner (SDK) messages will be returned.
func (tx Transaction) GetMsgs() []sdk.Msg {
	if tx.IsEmbeddedTx() {
		innerTx, err := tx.GetEmbeddedTx()
		if err != nil {
			panic(err)
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
	Messages   []sdk.Msg
	Signatures [][]byte
}

// Implement sdk.Tx interface
func (tx EmbeddedTx) GetMsgs() []sdk.Msg {
	return tx.Messages
}

// Return all required signers of Tx accumulated from msgs
func (tx EmbeddedTx) GetRequiredSigners() []ethcmn.Address {
	seen := map[string]bool{}

	var signers []ethcmn.Address
	for _, msg := range tx.GetMsgs() {
		for _, addr := range msg.GetSigners() {
			if !seen[addr.String()] {
				signers = append(signers, ethcmn.BytesToAddress(addr))
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
	AccountNumber int64
	Sequence int64
}

// Creates signBytes for signer with given arguments
func EmbeddedSignBytes(chainID string, msgs []sdk.Msg, sequence int64) []byte {
	var msgsBytes []json.RawMessage
	for _, msg := range msgs {
		msgsBytes = append(msgsBytes, json.RawMessage(msg.GetSignBytes()))
	}
	signDoc := EmbeddedSignDoc{
		ChainID:  chainID,
		Msgs:     msgsBytes,
		Sequence: sequence,
	}
	bz, err := Cdc.MarshalJSON(signDoc)
	if err != nil {
		panic(err)
	}
	return bz
}

func NewCodec() *wire.Codec {
	cdc := wire.NewCodec()
	cdc.RegisterInterface((*sdk.Msg)(nil), nil)
	cdc.RegisterConcrete(EmbeddedTx{}, "types/EmbeddedTx", nil)
	return cdc
}

// SetSDKAddress sets the internal sdkAddress value. It should ever be set
// once.
func SetSDKAddress(addr ethcmn.Address) {
	sdkAddressOnce.Do(func() {
		sdkAddress = addr
	})
}

func recoverSig(Vb, R, S, chainID  *big.Int) []byte {
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32 - len(r):32], r)
	copy(sig[64 - len(s):64], s)
	var v byte
	if chainID.Sign() == 0 {
		v = byte(Vb.Uint64() - 27)
	} else {
		chainIDMul := new(big.Int).Mul(chainID, big.NewInt(2))
		Vb.Sub(Vb, chainIDMul)
		v = byte(Vb.Uint64() - 35)
	}
	sig[64] = v
	return sig
}
