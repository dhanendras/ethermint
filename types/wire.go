package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
)

// RegisterWire registers all the necessary types with amino.
//
// TODO: We may need to redesign the registration process if the number and
// complexity of the types grows in order to provide better abstraction and
// encapsulation.
func RegisterWire(cdc *wire.Codec) {
	cdc.RegisterInterface((*sdk.Msg)(nil), nil)
	cdc.RegisterConcrete(InnerTransaction{}, "types/InnerTx", nil)
}
