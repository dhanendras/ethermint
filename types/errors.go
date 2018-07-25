package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Reserve this Codespace for Ethermint, as 0 and 1 are reserved by SDK
	DefaultCodespace sdk.CodespaceType = 2

	// Reserve CodeInvalidValue with first non-OK codetype
	CodeInvalidValue sdk.CodeType = 1
)

func codeToDefaultMsg(code sdk.CodeType) string {
	switch code {
	default:
		return sdk.CodeToDefaultMsg(code)
	}
}

//----------------------------------------
// Error constructors
func ErrInvalidValue(codespace sdk.CodespaceType, msg string) sdk.Error {
	return sdk.NewError(codespace, CodeInvalidValue, msg)
}
