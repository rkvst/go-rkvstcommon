package did

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"os"

	"github.com/datatrails/go-datatrails-common/logger"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/lestrrat-go/jwx/jwk"
)

/**
 * Keys handles the keys found as part of DID verificationMethod's
 */

// readRawECDSAPrivateKey reads a file containing a raw ecdsa private key in hex format and returns it
func readRawECDSAPrivateKey(keyFile string) (*ecdsa.PrivateKey, error) {

	pemKey, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	key, err := getRawECDSAPrivateKey(pemKey)
	if err != nil {
		return nil, err
	}

	return key, err

}

// getRawECDSAPrivateKey gets an ECDSA private key from a given hex byte slice.
func getRawECDSAPrivateKey(pemKey []byte) (*ecdsa.PrivateKey, error) {

	privateKey, err := ethcrypto.HexToECDSA(string(pemKey))
	if err != nil {
		return nil, err
	}

	return privateKey, err
}

// jwkToPublicKey converts a jwk key into a public key
// The following public keys are supported:
//
// 1. rsa
// 2. ecdsa
// 3. edwards
func jwkToPublicKey(jwkKey jwk.Key) (crypto.PublicKey, error) {

	// get the raw key
	var rawKey interface{}
	err := jwkKey.Raw(&rawKey)
	if err != nil {
		return nil, err
	}

	// attempt all the supported public key types
	rsaPublicKey, ok := rawKey.(*rsa.PublicKey)
	if ok {
		return rsaPublicKey, nil
	}

	ecdsaPublicKey, ok := rawKey.(*ecdsa.PublicKey)
	if ok {
		return ecdsaPublicKey, nil
	}

	edwardsPublicKey, ok := rawKey.(*ed25519.PublicKey)
	if ok {
		return edwardsPublicKey, nil
	}

	logger.Sugar.Infof("jwkToPublicKey: failed to convert to public key, unknown key type")
	return nil, ErrUnsupportedDIDKey
}
