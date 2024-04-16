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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/veraison/go-cose"
)

// IdentifiableCoseSigner represents a Cose1 signer that has additional methods to provide
// sufficient information to verify the signed product (an identifier for the signing key and the
// public key.)
type IdentifiableCoseSigner interface {
	cose.Signer
	PublicKey() (*ecdsa.PublicKey, error)
	KeyIdentifier() string
	KeyLocation() string
}

// IdentifiableCoseSignerFactory is for creating IdentifiableCoseSigners. The reason for a factory
// here is that we can always create a fresh instance, capturing the latest key information at that
// point in time.
type IdentifiableCoseSignerFactory interface {
	NewIdentifiableCoseSigner(ctx context.Context) (IdentifiableCoseSigner, error)
}

// KeyVaultCoseSignerFactory creates instances of our Azure KeyVault implementation of
// IdentifiableCoseSigner. The keyvault configuration is stored on the object and new instances
// can be created without caller knowledge of it.
type KeyVaultCoseSignerFactory struct {
	keyVaultURL string
	keyName     string
}

// NewKeyVaultCoseSignerFactory returns a new instance of the factory, storing the keyvault config
func NewKeyVaultCoseSignerFactory(keyVaultURL string, keyName string) *KeyVaultCoseSignerFactory {
	return &KeyVaultCoseSignerFactory{
		keyVaultURL: keyVaultURL,
		keyName:     keyName,
	}
}

// NewIdentifiableCoseSigner creates a new keyvault configuration that signs with ES384
// using the latest version of the named key
func (f *KeyVaultCoseSignerFactory) NewIdentifiableCoseSigner(ctx context.Context) (IdentifiableCoseSigner, error) {
	hasher := sha256.New()
	hasher.Write([]byte("azure-keyvault"))
	locationHash := hasher.Sum(nil)
	locationHashString := hex.EncodeToString(locationHash)[:6]

	kv := KeyVaultCoseSigner{
		ctx:     ctx,
		keyName: f.keyName,
		alg:     keyvault.ES384, // hardwired for the moment, add caller setting when needed
		KeyVault: &KeyVault{
			url: f.keyVaultURL,
		},
		locationIdentifier: locationHashString,
	}

	key, err := kv.GetLatestKey(ctx, kv.keyName)
	if err != nil {
		return nil, fmt.Errorf("unable to get latest key for %s: %w", kv.keyName, err)
	}

	if key.Key.Kid == nil {
		return nil, fmt.Errorf("key ID is nil")
	}

	kv.key = key

	return &kv, nil
}

// KeyVaultCoseSigner is the azure keyvault client for interacting with keyvault keys
// using a cose.Signer interface
type KeyVaultCoseSigner struct {
	// "We should not "Contexts should not be stored inside a struct type, but instead
	// passed to each function that needs it."
	// In this case this struct is transient and request scoped and the interface to the
	// methods does not include a context (and is outside of our control being defined
	// in the cose package).  We need the context for the logging ang tracing span.
	ctx                context.Context
	keyName            string
	alg                keyvault.JSONWebKeySignatureAlgorithm
	key                keyvault.KeyBundle
	locationIdentifier string
	*KeyVault
}

// KeyLocation returns an identifier for the place where the key is stored, used by the
// KeyIdentifier implementation.
func (kv *KeyVaultCoseSigner) KeyLocation() string {
	return kv.locationIdentifier
}

// Algorithm gets the cose algorithm for the key
func (kv *KeyVaultCoseSigner) Algorithm() cose.Algorithm {
	return cose.AlgorithmES384
}

// KeyIdentifier returns the essential information to identify the key, apart from any platform
// specific format (i.e. the Azure URL.) It takes the form: <location>:<key name>/<key version>.
// The location helps us identify where this key is stored. In this case, its azure key vault
func (kv *KeyVaultCoseSigner) KeyIdentifier() string {
	return fmt.Sprintf("%s:%s/%s", kv.KeyLocation(), GetKeyName(*kv.key.Key.Kid), GetKeyVersion(*kv.key.Key.Kid))
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

// PublicKey returns the public key for this instance of CoseSignerKeyVault
func (kv *KeyVaultCoseSigner) PublicKey() (*ecdsa.PublicKey, error) {
	if kv.key.Key.X == nil || kv.key.Key.Y == nil {
		return nil, fmt.Errorf("public key is nil")
	}

	X, err := base64BEtoBigInt(*kv.key.Key.X)
	if err != nil {
		return nil, fmt.Errorf("unable to convert X %s: %w", *kv.key.Key.X, err)
	}

	Y, err := base64BEtoBigInt(*kv.key.Key.Y)
	if err != nil {
		return nil, fmt.Errorf("unable to convert Y %s: %w", *kv.key.Key.Y, err)
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
func (kv *KeyVaultCoseSigner) Sign(rand io.Reader, content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(kv.ctx, 30*time.Second)
	defer cancel()

	signature, err := kv.KeyVault.HashAndSign(
		ctx,
		content,
		*kv.key.Key.Kid,
		kv.alg,
	)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to sign: %w", err)
	}

	return signature, nil
}
