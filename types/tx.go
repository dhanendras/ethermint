package types

import (
	"bytes"
	"math/big"
	"sync"
	"sync/atomic"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	sdkAddress     ethcmn.Address
	sdkAddressOnce sync.Once
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

	// TODO: Do we actually need this?
	txDataMarshaling struct {
		AccountNonce hexutil.Uint64
		Price        *hexutil.Big
		GasLimit     hexutil.Uint64
		Amount       *hexutil.Big
		Payload      hexutil.Bytes
		V            *hexutil.Big
		R            *hexutil.Big
		S            *hexutil.Big
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
func (tx Transaction) ConvertTx() ethtypes.Transaction {
	ethTx := ethtypes.NewTransaction(
		tx.data.AccountNonce, *tx.data.Recipient, tx.data.Amount,
		tx.data.GasLimit, tx.data.Price, tx.data.Payload,
	)

	// Must somehow set ethTx.data.{V, R, S}
	// Currently the only idea I have is to make ConvertTx take in a signer
	// Depending on signer type, reconstruct sig []byte from V, R, S
	// Call ethTx.WithSignature(signer, sig)
	// Not ideal.

	return *ethTx
}

// GetMsgs implements the Cosmos sdk.Tx interface. If the to/recipient address
// is the SDK address, the inner (SDK) messages will be returned.
func (tx Transaction) GetMsgs() []sdk.Msg {
	if bytes.Equal(tx.data.Recipient.Bytes(), sdkAddress.Bytes()) {
		innerTx, err := tx.GetInnerTx()
		if err != nil {
			// TODO: Should we panic here?
			return nil
		}

		return innerTx.GetMsgs()
	}

	return []sdk.Msg{tx}
}

// GetInnerTx returns the inner (SDK) transaction from an Ethereum transaction.
// It returns an error if decoding the inner transaction fails.
//
// CONTRACT: The payload field of an Ethereum transaction must contain a valid
// encoded SDK transaction.
func (tx Transaction) GetInnerTx() (InnerTransaction, sdk.Error) {
	// TODO: Fix...
	cdc := wire.NewCodec()
	innerTx := InnerTransaction{}

	err := cdc.UnmarshalBinary(tx.data.Payload, &innerTx)
	if err != nil {
		return InnerTransaction{}, sdk.ErrTxDecode("inner transaction decoding failed")
	}

	return innerTx, nil
}

// InnerTransaction reflects an SDK transaction. It is to be encoded into the
// payload field of an Ethereum transaction in order to route and handle SDK
// transactions.
type InnerTransaction struct {
	Messages   []sdk.Msg
	Signatures [][]byte
}

// GetMsgs implements the sdk.Tx interface. It returns all the SDK transaction
// messages.
func (tx InnerTransaction) GetMsgs() []sdk.Msg {
	return tx.Messages
}

// GetRequiredSigners returns all the required signers of an SDK transaction
// accumulated from messages. It returns them in a deterministic fashion given
// a list of messages.
func (tx InnerTransaction) GetRequiredSigners() []ethcmn.Address {
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

// SetSDKAddress sets the internal sdkAddress value. It should ever be set
// once.
func SetSDKAddress(addr ethcmn.Address) {
	sdkAddressOnce.Do(func() {
		sdkAddress = addr
	})
}
