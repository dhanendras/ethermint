package types

import (
	"crypto/ecdsa"

	ethcmn "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// GenerateAddress generates Ethereum Address
func GenerateAddress() ethcmn.Address {
	priv, err := ethcrypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	return PrivKeyToAddress(priv)
}

// PrivKeyToAddress takes ecdsa Privatekey and converts to Ethereum Address
func PrivKeyToAddress(p *ecdsa.PrivateKey) ethcmn.Address {
	return ethcrypto.PubkeyToAddress(ecdsa.PublicKey(p.PublicKey))
}
