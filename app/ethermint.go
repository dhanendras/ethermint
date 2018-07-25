package app

import (
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/ethermint/auth"
	"github.com/cosmos/ethermint/types"
	"github.com/cosmos/ethermint/db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	dbm "github.com/tendermint/tendermint/libs/db"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	appName = "Ethermint"
)

// EthermintApp implements an extended ABCI application.
type EthermintApp struct {
	*bam.BaseApp

	codec  *wire.Codec
	sealed bool

	// TODO: stores and keys
	accountKey *sdk.KVStoreKey
	
	// TODO: keepers

	// TODO: mappers
	accountMapper db.AccountMapper
}

// NewEthermintApp returns a reference to a new initialized Ethermint
// application.
func NewEthermintApp(logger log.Logger, db dbm.DB, config *params.ChainConfig, sdkAddress common.Address, opts ...func(*EthermintApp)) *EthermintApp {

	// Create codec here and register structs/interfaces in types using RegisterAmino(cdc)
	cdc := types.NewCodec()

	app := &EthermintApp{
		BaseApp: bam.NewBaseApp(appName, cdc, logger, db),
		codec:   cdc,
		accountKey: sdk.NewKVStoreKey("accounts")
	}

	app.accountMapper = db.NewAccountMapper(accountKey, cdc)

	// SetSDKAddress
	types.SetSDKAddress(sdkAddress)

	app.SetAnteHandler(auth.EthAnteHandler(config, sdkAddress))

	app.MountStoresIAVL(app.accountKey)
	
	for _, opt := range opts {
		opt(app)
	}

	err := app.LoadLatestVersion(accountKey)
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
