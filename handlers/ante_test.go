package handlers

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/x/auth"

	"github.com/cosmos/ethermint/types"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

func TestBadSignature(t *testing.T) {
	ms, handler := CreateHandler()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "2"}, false, log.NewNopLogger())

	// Create transaction with no signature
	tx := types.NewTransaction(0, types.GenerateAddress(), big.NewInt(1), 100, big.NewInt(3), []byte("My test bytes"))

	// Run Antehandler
	_, res, abort := handler(ctx, tx)

	// Assert antehandler failed
	assert.True(t, abort, "Transaction without signature did not abort")
	require.False(t, res.IsOK(), "Transaction did not fail with correct code")
}

func TestBadChainID(t *testing.T) {
	ms, handler := CreateHandler()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "2"}, false, log.NewNopLogger())

	// Create transaction with no signature
	tx := types.NewTransaction(0, types.GenerateAddress(), big.NewInt(1), 100, big.NewInt(3), []byte("My test bytes"))

	// Create signature with wrong chainID
	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	// ChainID 5 instead of 2
	tx.Sign(big.NewInt(5), privKey)

	// Run Antehandler
	_, res, abort := handler(ctx, tx)

	// Assert antehandler failed
	require.True(t, abort, "Transaction without signature did not abort")
	require.False(t, res.IsOK(), "Transaction did not fail with correct code")
}

func TestGoodTx(t *testing.T) {
	ms, handler := CreateHandler()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "2"}, false, log.NewNopLogger())

	// Create transaction with no signature
	tx := types.NewTransaction(0, types.GenerateAddress(), big.NewInt(1), 100, big.NewInt(3), []byte("My test bytes"))

	// Sign transaction
	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	tx.Sign(big.NewInt(2), privKey)

	// Run Antehandler
	_, res, abort := handler(ctx, *tx)

	// Assert antehandler passed
	assert.False(t, abort, "Valid Transaction aborted")
	require.True(t, res.IsOK(), res.Log)
}

func TestBadEmbeddedTx(t *testing.T) {
	ms, handler := CreateHandler()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "2"}, false, log.NewNopLogger())

	// Set reserved address
	reserved := types.GenerateAddress()
	types.SetSDKAddress(reserved)

	// Create transaction with no signature
	tx := types.NewTransaction(0, reserved, big.NewInt(1), 100, big.NewInt(3), []byte("Poorly embedded tx"))

	// Sign transaction
	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	tx.Sign(big.NewInt(2), privKey)

	// Run Antehandler
	_, res, abort := handler(ctx, *tx)

	// Assert antehandler failed
	require.True(t, abort, "Transaction with bad embedded tx did not abort")
	require.False(t, res.IsOK(), "Transaction did not fail with correct code")
}

func TestGoodEmbeddedTx(t *testing.T) {
	ms, handler := CreateHandler()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "2"}, false, log.NewNopLogger())

	// create codec
	cdc := wire.NewCodec()
	types.RegisterWire(cdc)
	payload := cdc.MustMarshalBinary([]sdk.Msg(nil))

	// Set reserved address
	reserved := types.GenerateAddress()
	types.SetSDKAddress(reserved)

	// Create transaction with no signature
	tx := types.NewTransaction(0, reserved, big.NewInt(1), 100, big.NewInt(3), payload)

	// Sign transaction
	privKey, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	tx.Sign(big.NewInt(2), privKey)

	// Run Antehandler
	_, res, abort := handler(ctx, *tx)

	// Assert antehandler passed
	assert.False(t, abort, "Valid Embedded Transaction aborted")
	require.True(t, res.IsOK(), res.Log)
}

func CreateHandler() (sdk.MultiStore, sdk.AnteHandler) {
	reserved := types.GenerateAddress()
	ms, addrKey := SetupMultiStore()

	cdc := wire.NewCodec()
	types.RegisterWire(cdc)

	accountMapper := auth.NewAccountMapper(cdc, addrKey, auth.ProtoBaseAccount)

	return ms, EthAnteHandler(reserved, accountMapper)
}
