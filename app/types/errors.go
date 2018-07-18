package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	DefaultCodespace sdk.CodespaceType = 2

	CodeInvalidValue      sdk.CodeType = 101
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