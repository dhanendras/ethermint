package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
)

var codec = wire.NewCodec()

func init() {
	RegisterWire(codec)
}

// RegisterWire registers all the necessary types with amino for the given
// codec.
//
// TODO: We may need to redesign the registration process if the number and
// complexity of the types grows in order to provide better abstraction and
// encapsulation.
func RegisterWire(codec *wire.Codec) {
	codec.RegisterInterface((*sdk.Msg)(nil), nil)
	codec.RegisterConcrete(EmbeddedTx{}, "types/EmbeddedTx", nil)
}
