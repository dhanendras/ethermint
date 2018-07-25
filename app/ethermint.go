package app

import (
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/wire"

	"github.com/cosmos/ethermint/auth"
	edb "github.com/cosmos/ethermint/db"
	"github.com/cosmos/ethermint/types"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethparams "github.com/ethereum/go-ethereum/params"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	appName = "Ethermint"
)

type (
	// EthermintApp implements an extended ABCI application. It is an application
	// that may process transactions through Ethereum's EVM running atop of
	// Tendermint consensus.
	EthermintApp struct {
		*bam.BaseApp

		codec  *wire.Codec
		sealed bool

		// TODO: stores and keys
		accountKey *sdk.KVStoreKey

		// TODO: keepers

		// TODO: mappers
		accountMapper edb.AccountMapper

		// TODO: stores and keys

		// TODO: keepers

		// TODO: mappers
	}

	// Options is a function signature that provides the ability to modify
	// options of an EthermintApp during initialization.
	Options func(*EthermintApp)
)

// NewEthermintApp returns a reference to a new initialized Ethermint
// application.
func NewEthermintApp(logger log.Logger, db dbm.DB, cfg *ethparams.ChainConfig, sdkAddr ethcmn.Address, opts ...Options) *EthermintApp {
	cdc := createCodec()

	app := &EthermintApp{
		BaseApp:    bam.NewBaseApp(appName, cdc, logger, db),
		codec:      cdc,
		accountKey: sdk.NewKVStoreKey("accounts"),
	}

	app.accountMapper = edb.NewAccountMapper(app.accountKey, cdc)

	// SetSDKAddress
	types.SetSDKAddress(sdkAddr)
	app.SetAnteHandler(auth.EthAnteHandler(cfg, sdkAddr, app.accountMapper))

	app.MountStoresIAVL(app.accountKey)

	for _, opt := range opts {
		opt(app)
	}

	err := app.LoadLatestVersion(app.accountKey)
	if err != nil {
		cmn.Exit(err.Error())
	}

	app.seal()
	return app
}

// seal seals the Ethermint application and prohibits any future modifications
// that change critical components.
func (app *EthermintApp) seal() {
	app.sealed = true
}

// createCodec creates a new amino wire codec and registers all the necessary
// structures and interfaces needed for the application.
func createCodec() *wire.Codec {
	var cdc = wire.NewCodec()

	types.RegisterWire(cdc)
	return cdc
}
