package azkeys

/**
 * KeyVault implements the azure keyvault API:
 *
 *  	https://learn.microsoft.com/en-us/rest/api/keyvault/
 */

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/datatrails/go-datatrails-common/logger"
)

// KeyVault is the azure keyvault client for interacting with keyvault keys
type KeyVault struct {
	url        string
	Authorizer autorest.Authorizer // optional, nil for production
}

// NewKeyVault creates a new keyvault client
func NewKeyVault(keyvaultURL string) *KeyVault {
	kv := KeyVault{
		url: keyvaultURL,
	}

	return &kv
}

// GetKeyByKID gets the key by its KID
func (kv *KeyVault) GetKeyByKID(
	ctx context.Context, keyID string,
) (keyvault.KeyBundle, error) {

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return keyvault.KeyBundle{}, err
	}

	keyName := GetKeyName(keyID)
	keyVersion := GetKeyVersion(keyID)

	key, err := kvClient.GetKey(ctx, kv.url, keyName, keyVersion)
	if err != nil {
		return keyvault.KeyBundle{}, fmt.Errorf("failed to read key: %w", err)
	}

	return key, nil

}

// GetLatestKey returns the latest version of the identified key
func (kv *KeyVault) GetLatestKey(
	ctx context.Context, keyName string,
) (keyvault.KeyBundle, error) {

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return keyvault.KeyBundle{}, err
	}

	key, err := kvClient.GetKey(ctx, kv.url, keyName, "")
	if err != nil {
		return keyvault.KeyBundle{}, fmt.Errorf("failed to read key: %w", err)
	}

	return key, nil
}

// GetKeyVersionsKeys returns all the keys, for all the versions of the identified key.
//
// The keys returned are the public half of the asymetric keys
func (kv *KeyVault) GetKeyVersionsKeys(
	ctx context.Context, keyID string,
) ([]keyvault.KeyBundle, error) {

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return []keyvault.KeyBundle{}, err
	}

	pageLimit := int32(1)
	keyVersions, err := kvClient.GetKeyVersions(ctx, kv.url, keyID, &pageLimit)
	if err != nil {
		return []keyvault.KeyBundle{}, fmt.Errorf("failed to read key: %w", err)
	}

	keyVersionValues := keyVersions.Values()

	keys, err := kv.getKeysFromVersions(ctx, keyVersionValues)
	if err != nil {
		log.Infof("failed to get key versions keys: %v", err)
		return []keyvault.KeyBundle{}, err
	}

	for keyVersions.NotDone() {
		err := keyVersions.NextWithContext(ctx)
		if err != nil {
			log.Infof("failed to get key versions: %v", err)
			return []keyvault.KeyBundle{}, err
		}

		keyVersionValues = keyVersions.Values()

		nextKeys, err := kv.getKeysFromVersions(ctx, keyVersionValues)
		if err != nil {
			log.Infof("failed to get next key versions keys: %v", err)
			return []keyvault.KeyBundle{}, err
		}

		keys = append(keys, nextKeys...)
	}

	return keys, nil
}

// getKeysFromVersions gets the keys from the given key versions
func (kv *KeyVault) getKeysFromVersions(ctx context.Context, keyVersions []keyvault.KeyItem) ([]keyvault.KeyBundle, error) {

	keys := []keyvault.KeyBundle{}

	for _, keyVersionValue := range keyVersions {

		// if we don't have a kid we can't find the key
		if keyVersionValue.Kid == nil {
			continue
		}

		key, err := kv.GetKeyByKID(ctx, *keyVersionValue.Kid)
		if err != nil {
			return []keyvault.KeyBundle{}, fmt.Errorf("failed get key version: %w", err)
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// GetKeyVersion gets the version of the given key
func GetKeyVersion(keyID string) string {

	// the kid is comprised of the {name}/{version}
	kidParts := strings.Split(keyID, "/")

	// get the version part
	return kidParts[len(kidParts)-1]

}

// GetKeyName gets the name of the given key
func GetKeyName(keyID string) string {

	// the kid is comprised of the {name}/{version}
	kidParts := strings.Split(keyID, "/")

	// get the name part
	return kidParts[len(kidParts)-2]
}

// Sign signs a given payload
func (kv *KeyVault) Sign(
	ctx context.Context,
	payload []byte,
	keyID string,
	algorithm keyvault.JSONWebKeySignatureAlgorithm,
) ([]byte, error) {

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create keyvault client: %w", err)
	}

	payloadStr := base64.URLEncoding.EncodeToString(payload)

	params := keyvault.KeySignParameters{
		Algorithm: algorithm,
		Value:     &payloadStr,
	}
	keyName := GetKeyName(keyID)
	keyVersion := GetKeyVersion(keyID)

	signatureb64, err := kvClient.Sign(ctx, kv.url, keyName, keyVersion, params)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to sign payload: %w", err)
	}

	signature, err := base64.URLEncoding.DecodeString(*signatureb64.Result)

	return signature, err
}

// hashPayload hashes the payload depending on what the keytype is
func hashPayload(toBeSigned []byte, key keyvault.KeyBundle) ([]byte, error) {

	switch key.Key.Kty {
	case keyvault.EC:
		switch key.Key.Crv {
		case keyvault.P256:
			toBeSignedHash := sha256.Sum256(toBeSigned)
			return toBeSignedHash[:], nil
		case keyvault.P384:
			toBeSignedHash := sha512.Sum384(toBeSigned)
			return toBeSignedHash[:], nil
		default:
			return nil, errors.New("unsupported key")
		}
	default:
		return nil, errors.New("unsupported key")
	}
}

// Sign signs a hash of a given payload
func (kv *KeyVault) HashAndSign(
	ctx context.Context,
	payload []byte,
	keyID string,
	algorithm keyvault.JSONWebKeySignatureAlgorithm,
) ([]byte, error) {

	key, err := kv.GetKeyByKID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	payloadHash, err := hashPayload(payload, key)
	if err != nil {
		return nil, fmt.Errorf("failed to hash payload: %w", err)
	}

	signature, err := kv.Sign(ctx, payloadHash, keyID, algorithm)
	if err != nil {
		return nil, fmt.Errorf("failed to sign hash: %w", err)
	}

	return signature, nil
}

// Verify verifies a given payload
func (kv *KeyVault) Verify(
	ctx context.Context,
	signature []byte,
	digest []byte,
	keyID string,
	keyVersion string,
	algorithm keyvault.JSONWebKeySignatureAlgorithm,
) (bool, error) {

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return false, fmt.Errorf("failed to create keyvault client: %w", err)
	}

	signatureStr := base64.URLEncoding.EncodeToString(signature)
	digestStr := base64.URLEncoding.EncodeToString(digest)

	params := keyvault.KeyVerifyParameters{
		Algorithm: algorithm,
		Signature: &signatureStr,
		Digest:    &digestStr,
	}

	result, err := kvClient.Verify(ctx, kv.url, keyID, keyVersion, params)
	if err != nil {
		return false, fmt.Errorf("failed to verify payload: %w", err)
	}
	return *result.Value, err

}
