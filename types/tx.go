package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	// TypeTxEthereum reflects an Ethereum Transaction type.
	TypeTxEthereum = "Ethereum"
)

var (
	sdkAddress     ethcmn.Address
	sdkAddressOnce sync.Once
)

// SetSDKAddress sets the internal sdkAddress value. It should ever be set
// once.
func SetSDKAddress(addr ethcmn.Address) {
	sdkAddressOnce.Do(func() {
		sdkAddress = addr
	})
}

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

// NewTransaction mimics ethereum's NewTransaction method
func NewTransaction(nonce uint64, to ethcmn.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	if len(data) > 0 {
		data = ethcmn.CopyBytes(data)
	}
	d := TxData{
		AccountNonce: nonce,
		Recipient:    &to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &Transaction{data: d}
}

// TxData returns the Ethereum transaction data.
func (tx Transaction) TxData() TxData {
	return tx.data
}

// Sign takes the private key and chainID to sign Ethereum transaction
// according to EIP155 standard. Mutates transaction to populate V, R, S fields.
func (tx *Transaction) Sign(chainID *big.Int, priv *ecdsa.PrivateKey) {
	h := rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		chainID, uint(0), uint(0),
	})

	sig, err := crypto.Sign(h[:], priv)
	if err != nil {
		panic(err)
	}

	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])

	var v *big.Int
	if chainID.Sign() == 0 {
		v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	} else {
		v = big.NewInt(int64(sig[64] + 35))
		chainIDMul := new(big.Int).Mul(chainID, big.NewInt(2))
		v.Add(v, chainIDMul)
	}

	tx.data.V = v
	tx.data.R = r
	tx.data.S = s
}

// Type implements the sdk.Msg interface. It returns the type of the
// Transaction.
func (tx Transaction) Type() string { return TypeTxEthereum }

// ValidateBasic implements the sdk.Msg interface. It performs basic validation
// checks of a Transaction. If returns an sdk.Error if validation fails.
func (tx Transaction) ValidateBasic() sdk.Error {
	if tx.data.Price.Sign() != 1 {
		return ErrInvalidValue(DefaultCodespace, "price must be positive")
	}

	if tx.data.Amount.Sign() != 1 {
		return ErrInvalidValue(DefaultCodespace, "amount must be positive")
	}

	return nil
}

// GetSignBytes implements the sdk.Msg Interface. It returns nil as the bytes
// signed must include the chainID and sequence number.
func (tx Transaction) GetSignBytes() []byte {
	return nil
}

// GetSigners implements the Cosmos sdk.Msg interface. It will return a single
// SDK account signer based on the from address.
//
// CONTRACT: The transaction must already be signed.
func (tx Transaction) GetSigners() []sdk.AccAddress {
	addr := tx.from.Load().([]byte)
	if addr == nil {
		return nil
	}

	return []sdk.AccAddress{addr}
}

// ConvertTx attempts to converts a Transaction to a new Ethereum transaction
// with the signature set. The signature if first recovered and then a new
// Transaction is created with that signature. If setting the signature fails,
// a panic will be triggered.
func (tx Transaction) ConvertTx(chainID *big.Int) ethtypes.Transaction {
	ethTx := ethtypes.NewTransaction(
		tx.data.AccountNonce, *tx.data.Recipient, tx.data.Amount,
		tx.data.GasLimit, tx.data.Price, tx.data.Payload,
	)

	sig := recoverSig(tx.data.V, tx.data.R, tx.data.S, chainID)
	signer := ethtypes.NewEIP155Signer(chainID)

	ethTx, err := ethTx.WithSignature(signer, sig)
	if err != nil {
		panic(errors.Wrap(err, "failed to create new transaction with a given signature"))
	}

	return *ethTx
}

// HasEmbeddedTx returns a boolean reflecting if the transaction contains an
// SDK transaction or not based on the recipient address.
func (tx Transaction) HasEmbeddedTx() bool {
	return bytes.Equal(tx.data.Recipient.Bytes(), sdkAddress.Bytes())
}

// GetMsgs implements the Cosmos sdk.Tx interface. If the to/recipient address
// is the SDK address, the inner (SDK) messages will be returned.
func (tx Transaction) GetMsgs() []sdk.Msg {
	if tx.HasEmbeddedTx() {
		innerTx, err := tx.GetEmbeddedTx()
		if err != nil {
			panic(errors.Wrap(err, "failed to get embedded transaction"))
		}

		return innerTx.GetMsgs()
	}

	return []sdk.Msg{tx}
}

// GetEmbeddedTx returns the embedded SDK transaction from an Ethereum
// transaction. It returns an error if decoding the inner transaction fails.
//
// CONTRACT: The payload field of an Ethereum transaction must contain a valid
// encoded SDK transaction.
func (tx Transaction) GetEmbeddedTx() (EmbeddedTx, sdk.Error) {
	etx := EmbeddedTx{}

	err := codec.UnmarshalBinary(tx.data.Payload, &etx)
	if err != nil {
		return EmbeddedTx{}, sdk.ErrTxDecode("embedded sdk transaction decoding failed")
	}

	return etx, nil
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(ethcmn.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// EmbeddedTx implements an SDK transaction. It is to be encoded into the
// payload field of an Ethereum transaction in order to route and handle SDK
// transactions.
type EmbeddedTx struct {
	Messages   []sdk.Msg
	Signatures [][]byte
}

// GetMsgs implements the sdk.Tx interface. It returns all the SDK transaction
// messages.
func (tx EmbeddedTx) GetMsgs() []sdk.Msg {
	return tx.Messages
}

// GetRequiredSigners returns all the required signers of an SDK transaction
// accumulated from messages. It returns them in a deterministic fashion given
// a list of messages.
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

// SignBytes creates signature bytes for a signer to sign. The signature bytes
// require a chainID and an account number. The signature bytes are JSON
// encoded.
func (tx EmbeddedTx) SignBytes(chainID string, accnum, sequence int64) []byte {
	var msgsBytes []json.RawMessage
	for _, msg := range tx.GetMsgs() {
		msgsBytes = append(msgsBytes, json.RawMessage(msg.GetSignBytes()))
	}

	signDoc := EmbeddedSignDoc{
		ChainID:       chainID,
		Msgs:          msgsBytes,
		AccountNumber: accnum,
		Sequence:      sequence,
	}

	bz, err := codec.MarshalJSON(signDoc)
	if err != nil {
		panic(err)
	}

	return bz
}

// ValidateBasic performs basic validation checks of an EmbeddedTx. If returns
// an sdk.Error if validation fails.
func (tx EmbeddedTx) ValidateBasic() sdk.Error {
	signers := tx.GetRequiredSigners()

	if len(tx.Signatures) != len(signers) {
		return sdk.ErrUnauthorized("provided signature length does not match required length")
	}

	for _, msg := range tx.GetMsgs() {
		if msg.Type() == TypeTxEthereum {
			return sdk.ErrTxDecode("invalid embedded message; cannot have Ethereum transaction in EmbeddedTx")
		}

		if err := msg.ValidateBasic(); err != nil {
			return err
		}
	}

	return nil
}

// EmbeddedSignDoc implements a simple SignDoc for a EmbeddedTx signer to sign
// over.
type EmbeddedSignDoc struct {
	ChainID       string
	Msgs          []json.RawMessage
	AccountNumber int64
	Sequence      int64
}

// recoverSig recovers a signature according to the Ethereum specification.
func recoverSig(Vb, R, S, chainID *big.Int) []byte {
	var v byte

	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)

	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)

	if chainID.Sign() == 0 {
		v = byte(Vb.Uint64() - 27)
	} else {
		chainIDMul := new(big.Int).Mul(chainID, big.NewInt(2))
		V := new(big.Int).Sub(Vb, chainIDMul)

		v = byte(V.Uint64() - 35)
	}

	sig[64] = v
	return sig
}

func rlpHash(x interface{}) (h ethcmn.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
