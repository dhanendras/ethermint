package auth

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ethermint/types"
	"github.com/cosmos/ethermint/db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"reflect"
	"fmt"
)

// AnteHandler to be passed into baseapp
// Handles Ethereum transactions and passes SDK transactions to InnerAnteHandler
func EthAnteHandler(config *params.ChainConfig, sdkAddress common.Address, accountMapper db.AccountMapper) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx) (newCtx sdk.Context, res sdk.Result, abort bool) {

		transact, ok := tx.(types.Transaction)
		if !ok {
			return ctx, sdk.ErrInternal("tx must be an Ethereum transaction").Result(), true
		}

		txdata := transact.TxData()

		newCtx = ctx.WithGasMeter(sdk.NewGasMeter(int64(txdata.GasLimit)))

		// AnteHandlers must have their own defer/recover in order
		// for the BaseApp to know how much gas was used!
		// This is because the GasMeter is created in the AnteHandler,
		// but if it panics the context won't be set properly in runTx's recover ...
		defer func() {
			if r := recover(); r != nil {
				switch rType := r.(type) {
				case sdk.ErrorOutOfGas:
					log := fmt.Sprintf("out of gas in location: %v", rType.Descriptor)
					res = sdk.ErrOutOfGas(log).Result()
					res.GasWanted = int64(txdata.GasLimit)
					res.GasUsed = newCtx.GasMeter().GasConsumed()
					abort = true
				default:
					panic(r)
				}
			}
		}()

		ethTx := transact.ConvertTx()

		// Create correct signer based on config and blockheight
		signer := ethTypes.MakeSigner(config, big.NewInt(ctx.BlockHeight()))

		// Check that signature is valid. Maybe better way to do this?
		// TODO: Maybe we should increment AccountNonce in mapper here as well?
		_, err := signer.Sender(&ethTx)
		if err != nil {
			return ctx, sdk.ErrUnauthorized("Signature verification failed").Result(), true
		}


		if reflect.DeepEqual(*ethTx.To(), sdkAddress) {
			embeddedTx, err := transact.GetEmbeddedTx()
			if err != nil {
				return ctx, err.Result(), true
			}
			return EmbeddedAnteHandler(newCtx, embeddedTx, accountMapper)
		}

		// Handle Normal ETH transaction
		return ctx, sdk.Result{}, false
	}
}

// Since this is an internal antehandler, does not need to follow interface
// We can change function signature if we want
func EmbeddedAnteHandler(ctx sdk.Context, tx types.EmbeddedTx, accountMapper db.AccountMapper) (_ sdk.Context, _ sdk.Result, abort bool) {
	// Validate basic tx without relying on context
	if err := validateBasic(tx); err != nil {
		return ctx, err.Result(), true
	}

	// Validate Signatures
	sigs := tx.Signatures
	signerAddrs := tx.GetRequiredSigners()
	msgs := tx.GetMsgs()

	for i, sig := range sigs {
		signer := signerAddrs[i]
		seq, err := accountMapper.GetSequence(ctx, signer)
		if err != nil {
			return ctx, err.Result(), true
		}

		signBytes := types.EmbeddedSignBytes(ctx.ChainID(), msgs, seq)
		pk, err2 := crypto.SigToPub(signBytes, sig)
		if crypto.PubkeyToAddress(*pk) != signer || err2 != nil {
			return ctx, sdk.ErrUnauthorized("signature verification failed").Result(), true
		}
		incrementSequenceNumber(ctx, accountMapper, signer)
	}

	return ctx, sdk.Result{}, false
}

func validateBasic(tx types.EmbeddedTx) sdk.Error {
	signers := tx.GetRequiredSigners()

	if len(tx.Signatures) != len(signers) {
		return sdk.ErrUnauthorized("Provided signature length does not match required amount")
	}

	for _, msg := range tx.GetMsgs() {
		// Do not allow types.Transaction to be embedded here
		if msg.Type() == "Eth" {
			return sdk.ErrTxDecode("Cannot have Eth transaction in EmbeddedTx")
		}
		// Must run validate basic on each msg here since baseapp doesn't know about inner msgs
		if err := msg.ValidateBasic(); err != nil {
			return err
		}
	}
	return nil
}

func incrementSequenceNumber(ctx sdk.Context, accountMapper db.AccountMapper, addr common.Address) {
	acc := accountMapper.GetAccount(ctx, addr)
	acc.AccountNonce += 1
	accountMapper.SetAccount(ctx, acc)
}
