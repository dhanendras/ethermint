package auth

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ethermint/types"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"reflect"
)

// AnteHandler to be passed into baseapp
// Handles Ethereum transactions and passes SDK transactions to InnerAnteHandler
func EthAnteHandler(config *params.ChainConfig, sdkAddress common.Address) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx) (_ sdk.Context, _ sdk.Result, abort bool) {

		transact, ok := tx.(types.Transaction)
		if !ok {
			return ctx, sdk.ErrInternal("tx must be an Ethereum transaction").Result(), true
		}

		txdata := transact.TxData()

		ctx = ctx.WithGasMeter(sdk.NewGasMeter(int64(txdata.GasLimit)))

		ethTx := transact.ConvertTx()

		// Create correct signer based on config and blockheight
		signer := ethTypes.MakeSigner(config, big.NewInt(ctx.BlockHeight()))

		// Check that signature is valid. Maybe better way to do this?
		_, err := signer.Sender(&ethTx)
		if err != nil {
			return ctx, sdk.ErrUnauthorized("Signature verification failed").Result(), true
		}

		if reflect.DeepEqual(*ethTx.To(), sdkAddress) {
			innerTx, err := transact.GetInnerTx()
			if err != nil {
				return ctx, err.Result(), true
			}
			return InnerAnteHandler(ctx, innerTx)
		}

		// Handle Normal ETH transaction
		return ctx, sdk.Result{}, false
	}
}

// Since this is an internal antehandler, does not need to follow interface
// We can change function signature if we want
func InnerAnteHandler(ctx sdk.Context, tx types.InnerTransaction) (_ sdk.Context, _ sdk.Result, abort bool) {

	signers := tx.GetRequiredSigners()

	if len(tx.Signatures) != len(signers)-1 {
		return ctx, sdk.ErrUnauthorized("Provided signature length does not match required amount").Result(), true
	}

	// Must run validate basic on each msg here since baseapp doesn't know about inner msgs
	for _, msg := range tx.GetMsgs() {
		if err := msg.ValidateBasic(); err != nil {
			return ctx, sdk.NewError(2, 101, err.Error()).Result(), true
		}
	}

	// TODO: Validate Signatures
	// Will have to come up with SignDoc for inner tx's to include ChainID and maybe some other info

	return ctx, sdk.Result{}, false
}
