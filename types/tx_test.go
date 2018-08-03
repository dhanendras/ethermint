package types

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestConversion(t *testing.T) {
	ethTx, emintTx := TwinTransactions()

	recoverTx := emintTx.ConvertTx(big.NewInt(3))

	require.Equal(t, *ethTx, recoverTx, "Conversion failed")
}

func TestEncoding(t *testing.T) {
	ethTx, emintTx := TwinTransactions()

	// test that encoding of Ethereum transaction and Ethermint transaction is identical
	ethtxBytes, err1 := rlp.EncodeToBytes(ethTx)
	emintTxBytes, err2 := rlp.EncodeToBytes(emintTx)
	if err1 != nil {
		panic(err1)
	}
	if err2 != nil {
		panic(err2)
	}

	require.True(t, bytes.Equal(ethtxBytes, emintTxBytes), "Encoding ethTx and emintTx created different values")
}

func TestValidation(t *testing.T) {
	_, badTx := TwinTransactions()

	badTx.data.Price.Set(big.NewInt(-1))
	err := badTx.ValidateBasic()
	require.Equal(t, sdk.CodeType(1), err.Code())

	_, badTx = TwinTransactions()
	badTx.data.Amount.Set(big.NewInt(-1))
	require.Equal(t, sdk.CodeType(1), err.Code())
}

func TestEmbedded(t *testing.T) {
	reserved := GenerateAddress()
	SetSDKAddress(reserved)
	etx := EmbeddedTx{
		Messages:   []sdk.Msg(nil),
		Signatures: [][]byte{[]byte("sig1")},
	}
	payload := codec.MustMarshalBinary(etx)

	eData := TxData{
		Payload:   payload,
		Recipient: &reserved,
	}
	tx := Transaction{data: eData}

	require.True(t, tx.HasEmbeddedTx(), "Embedded Tx check unsuccessful")

	recoverTx, err := tx.GetEmbeddedTx()
	require.Nil(t, err, "Extraction returned error")
	require.Equal(t, etx, recoverTx, "Embedded tx extraction failed")
}

func TwinTransactions() (*ethtypes.Transaction, *Transaction) {
	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	addr := PrivKeyToAddress(privKey)
	ethTx := ethtypes.NewTransaction(1, addr, big.NewInt(10), 100, big.NewInt(100), []byte("My test bytes"))
	signer := ethtypes.NewEIP155Signer(big.NewInt(3))
	ethTx, err = ethtypes.SignTx(ethTx, signer, privKey)
	if err != nil {
		panic(err)
	}

	emintTx := NewTransaction(1, addr, big.NewInt(10), 100, big.NewInt(100), []byte("My test bytes"))
	emintTx.Sign(big.NewInt(3), privKey)

	return ethTx, emintTx
}
