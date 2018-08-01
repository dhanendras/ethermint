package handlers

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"

	"github.com/cosmos/ethermint/types"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	ethparams "github.com/ethereum/go-ethereum/params"

	"github.com/pkg/errors"
)

// EthAnteHandler handles Ethereum transactions and passes SDK transactions to
// InnerAnteHandler.
func EthAnteHandler(config *ethparams.ChainConfig, sdkAddress ethcmn.Address, accountMapper auth.AccountMapper) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx) (newCtx sdk.Context, res sdk.Result, abort bool) {
		mintTx, ok := tx.(types.Transaction)
		if !ok {
			return ctx, sdk.ErrInternal("tx must be an Ethereum transaction").Result(), true
		}

		txData := mintTx.TxData()
		newCtx = ctx.WithGasMeter(sdk.NewGasMeter(int64(txData.GasLimit)))

		// AnteHandlers must have their own defer/recover in order for the
		// BaseApp to know how much gas was used! This is because the GasMeter
		// is created in the AnteHandler, but if it panics the context won't be
		// set properly in runTx's recover.
		defer func() {
			if r := recover(); r != nil {
				switch rType := r.(type) {
				case sdk.ErrorOutOfGas:
					log := fmt.Sprintf("out of gas in location: %v", rType.Descriptor)
					res = sdk.ErrOutOfGas(log).Result()
					res.GasWanted = int64(txData.GasLimit)
					res.GasUsed = newCtx.GasMeter().GasConsumed()
					abort = true
				default:
					panic(r)
				}
			}
		}()

		// the SDK chainID is a string representation of integer
		chainID, ok := new(big.Int).SetString(ctx.ChainID(), 10)
		if !ok {
			// TODO: ErrInternal may not be correct error to throw here?
			return newCtx, sdk.ErrInternal("invalid chainID").Result(), true
		}

		ethTx := mintTx.ConvertTx(chainID)
		signer := ethtypes.NewEIP155Signer(chainID)

		// Check that signature is valid. Maybe better way to do this?
		// TODO: Maybe we should increment AccountNonce in mapper here as well?
		_, err := signer.Sender(&ethTx)
		if err != nil {
			return newCtx, sdk.ErrUnauthorized("signature verification failed").Result(), true
		}

		if mintTx.HasEmbeddedTx() {
			embeddedTx, err := mintTx.GetEmbeddedTx()
			if err != nil {
				return newCtx, err.Result(), true
			}

			return embeddedAnteHandler(newCtx, embeddedTx, accountMapper)
		}

		// handle normal Ethereum transaction
		return newCtx, sdk.Result{}, false
	}
}

// embeddedAnteHandler handles an embedded SDK transaction. Since this is an
// internal ante handler, it does not need to follow the SDK interface.
func embeddedAnteHandler(ctx sdk.Context, tx types.EmbeddedTx, am auth.AccountMapper) (sdk.Context, sdk.Result, bool) {
	if err := tx.ValidateBasic(); err != nil {
		return ctx, err.Result(), true
	}

	// validate signatures
	signerAddrs := tx.GetRequiredSigners()
	for i, sig := range tx.Signatures {
		signer := signerAddrs[i]

		// attempt to get the sequence number of the signer
		seq, err := am.GetSequence(ctx, signer.Bytes())
		if err != nil {
			return ctx, err.Result(), true
		}

		// TODO: Do we not need to also include the account number as part of
		// the data to sign?
		signBytes := tx.SignBytes(ctx.ChainID(), seq)

		if err := validateSigner(ctx, signBytes, sig, signer); err != nil {
			return ctx, sdk.ErrUnauthorized(err.Error()).Result(), true
		}

		if err := incrSequenceNumber(ctx, am, signer); err != nil {
			return ctx, sdk.ErrInternal(err.Error()).Result(), true
		}
	}

	return ctx, sdk.Result{}, false
}

// validateSigner attempts to validate a signer for a given slice of bytes over
// which a signature and signer is given. An error is returned if address
// derived from the signature and bytes signed does not match the given signer.
func validateSigner(ctx sdk.Context, signBytes, sig []byte, signer ethcmn.Address) error {
	pk, err := ethcrypto.SigToPub(signBytes, sig)
	if ethcrypto.PubkeyToAddress(*pk) != signer || err != nil {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// incrSequenceNumber attempts to increment the sequence number of a given
// account and updates the state. An error is returned if updating the state
// fails.
func incrSequenceNumber(ctx sdk.Context, am auth.AccountMapper, addr ethcmn.Address) error {
	acc := am.GetAccount(ctx, addr.Bytes())

	if err := acc.SetSequence(acc.GetSequence() + 1); err != nil {
		return errors.Wrap(err, "failed to update account sequence")
	}

	am.SetAccount(ctx, acc)
	return nil
}
