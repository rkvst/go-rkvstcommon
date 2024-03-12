package azkeys

/**
 * coseSigner implements the cose.Signer interface:
 *
 *  	https://github.com/veraison/go-cose
 */

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/veraison/go-cose"
)

// CoseSignerKeyVault is the azure keyvault client for interacting with keyvault keys
// using a cose.Signer interface
type CoseSignerKeyVault struct {
	keyName string
	alg     keyvault.JSONWebKeySignatureAlgorithm
	*KeyVault
}

// NewCoseSignerKeyVault creates a new keyvault configuration that signs with ES384
// using the latest version of the named key
func NewCoseSignerKeyVault(keyVaultURL string, keyName string) *CoseSignerKeyVault {
	kv := CoseSignerKeyVault{
		keyName: keyName,
		alg:     keyvault.ES384, // hardwired for the moment, add caller setting when needed
		KeyVault: &KeyVault{
			url: keyVaultURL,
		},
	}
	return &kv
}

// Algorithm gets the cose algorithm for the key
func (kv *CoseSignerKeyVault) Algorithm() cose.Algorithm {
	return cose.AlgorithmES384
}

// base64BEtoBigInt takes a URL Encoded, base 64 encoded big endian byte array
// and returns a big.Int
func base64BEtoBigInt(in string) (*big.Int, error) {
	s, err := base64.URLEncoding.DecodeString(in)
	if err != nil {
		return nil, fmt.Errorf("unable to base64 decode string %s: %w", in, err)
	}
	i := new(big.Int)
	// As the byte array is big endian no conversion is needed
	i.SetBytes(s)
	return i, nil
}

// PublicKey gets the latest key's public key
func (kv *CoseSignerKeyVault) PublicKey() (*ecdsa.PublicKey, error) {
	logger.Sugar.Infof("PublicKey: %s %s", kv.url, kv.keyName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key, err := kv.GetLatestKey(ctx, kv.keyName)
	if err != nil {
		return nil, fmt.Errorf("unable to get latest key for %s: %w", kv.keyName, err)
	}

	if key.Key.X == nil || key.Key.Y == nil {
		return nil, fmt.Errorf("public key is nil")
	}

	X, err := base64BEtoBigInt(*key.Key.X)
	if err != nil {
		return nil, fmt.Errorf("unable to convert X %s: %w", *key.Key.X, err)
	}

	Y, err := base64BEtoBigInt(*key.Key.Y)
	if err != nil {
		return nil, fmt.Errorf("unable to convert Y %s: %w", *key.Key.Y, err)
	}

	return &ecdsa.PublicKey{
		Curve: &elliptic.CurveParams{
			Name: "P-384",
		},
		X: X,
		Y: Y,
	}, nil
}

// Sign signs a given content
func (kv *CoseSignerKeyVault) Sign(rand io.Reader, content []byte) ([]byte, error) {

	logger.Sugar.Infof("Sign: %s %s", kv.url, kv.keyName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key, err := kv.GetLatestKey(ctx, kv.keyName)
	if err != nil {
		return []byte{}, fmt.Errorf("unable to get latest key for %s: %w", kv.keyName, err)
	}

	if key.Key.Kid == nil {
		return []byte{}, fmt.Errorf("key ID is nil")
	}

	signature, err := kv.KeyVault.HashAndSign(
		ctx,
		content,
		*key.Key.Kid,
		kv.alg,
	)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to sign: %w", err)
	}

	return signature, nil
}
