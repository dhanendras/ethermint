package auth

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ethermint/types"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"
)

// EthAnteHandler handles Ethereum transactions and passes SDK transactions to
// InnerAnteHandler.
func EthAnteHandler(config *ethparams.ChainConfig, sdkAddress ethcmn.Address) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx) (sdk.Context, sdk.Result, bool) {
		mintTx, ok := tx.(types.Transaction)
		if !ok {
			return ctx, sdk.ErrInternal("tx must be an Ethereum transaction").Result(), true
		}

		txData := mintTx.TxData()
		ctx = ctx.WithGasMeter(sdk.NewGasMeter(int64(txData.GasLimit)))
		ethTx := mintTx.ConvertTx()

		// create correct signer based on config and blockheight
		signer := ethtypes.MakeSigner(config, big.NewInt(ctx.BlockHeight()))

		// Check that signature is valid. Maybe better way to do this?
		_, err := signer.Sender(&ethTx)
		if err != nil {
			return ctx, sdk.ErrUnauthorized("signature verification failed").Result(), true
		}

		if mintTx.IsSDKTx() {
			innerTx, err := mintTx.GetInnerTx()
			if err != nil {
				return ctx, err.Result(), true
			}

			return InnerAnteHandler(ctx, innerTx)
		}

		// handle normal Ethereum transaction
		return ctx, sdk.Result{}, false
	}
}

// InnerAnteHandler implements the ante handler for an embedded SDK
// transaction. Since this is an internal ante handler, it does not need to
// follow the SDK interface.
func InnerAnteHandler(ctx sdk.Context, tx types.InnerTransaction) (sdk.Context, sdk.Result, bool) {
	signers := tx.GetRequiredSigners()

	if len(tx.Signatures) != len(signers)-1 {
		return ctx, sdk.ErrUnauthorized("provided signature length does not match required amount").Result(), true
	}

	// Must run validate basic on each msg here since the baseapp doesn't know
	// about inner messages.
	for _, msg := range tx.GetMsgs() {
		if err := msg.ValidateBasic(); err != nil {
			return ctx, sdk.NewError(2, 101, err.Error()).Result(), true
		}
	}

	// TODO: Validate Signatures
	// We will have to come up with SignDoc for inner tx's to include ChainID
	// and maybe some other info.

	return ctx, sdk.Result{}, false
}
