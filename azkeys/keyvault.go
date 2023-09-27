package azkeys

/**
 * KeyVault implements the azure keyvault API:
 *
 *  	https://learn.microsoft.com/en-us/rest/api/keyvault/
 */

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/rkvst/go-rkvstcommon/logger"
)

// KeyVault is the azure keyvault client for interacting with keyvault keys
type KeyVault struct {
	Name       string
	Authorizer autorest.Authorizer // optional, nil for production
}

// NewKeyVault creates a new keyvault client
func NewKeyVault(keyvaultURL string) *KeyVault {
	kv := KeyVault{
		Name: keyvaultURL,
	}

	return &kv
}

// GetKeyByKID gets the key by its KID
func (kv *KeyVault) GetKeyByKID(
	ctx context.Context, kid string,
) (keyvault.KeyBundle, error) {

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("GetLatestKey: %s %s", kv.Name, kid)

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return keyvault.KeyBundle{}, err
	}

	keyName := GetKeyName(kid)
	keyVersion := GetKeyVersion(kid)

	key, err := kvClient.GetKey(ctx, kv.Name, keyName, keyVersion)
	if err != nil {
		return keyvault.KeyBundle{}, fmt.Errorf("failed to read key: %w", err)
	}

	return key, nil

}

// GetLatestKey returns the latest version of the identified key
func (kv *KeyVault) GetLatestKey(
	ctx context.Context, keyID string,
) (keyvault.KeyBundle, error) {

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("GetLatestKey: %s %s", kv.Name, keyID)

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return keyvault.KeyBundle{}, err
	}

	key, err := kvClient.GetKey(ctx, kv.Name, keyID, "")
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

	log.Infof("GetKeyVersions: %s %s", kv.Name, keyID)

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return []keyvault.KeyBundle{}, err
	}

	pageLimit := int32(1)
	keyVersions, err := kvClient.GetKeyVersions(ctx, kv.Name, keyID, &pageLimit)
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

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("getKeysFromVersions")

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
func GetKeyVersion(kid string) string {

	// the kid is comprised of the {name}/{version}
	kidParts := strings.Split(kid, "/")

	// get the version part
	return kidParts[len(kidParts)-1]

}

// GetKeyName gets the name of the given key
func GetKeyName(kid string) string {

	// the kid is comprised of the {name}/{version}
	kidParts := strings.Split(kid, "/")

	// get the name part
	return kidParts[len(kidParts)-2]
}

// Sign signs a given payload
func (kv *KeyVault) Sign(
	ctx context.Context,
	payload []byte,
	keyID string,
	keyVersion string,
	algorithm keyvault.JSONWebKeySignatureAlgorithm,
) ([]byte, error) {

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("Sign: %s %s", kv.Name, keyID)

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return []byte{}, err
	}

	payloadStr := base64.URLEncoding.EncodeToString(payload)

	logger.Sugar.Infof("Payload Str: %v", payloadStr)

	params := keyvault.KeySignParameters{
		Algorithm: algorithm,
		Value:     &payloadStr,
	}

	signatureb64, err := kvClient.Sign(ctx, kv.Name, keyID, keyVersion, params)
	if err != nil {
		return []byte{}, fmt.Errorf("failed toado sign payl: %w", err)
	}

	logger.Sugar.Infof("SignatureB64: %v", *signatureb64.Result)
	signature, err := base64.URLEncoding.DecodeString(*signatureb64.Result)
	return signature, err

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

	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("Verify: %s %s", kv.Name, keyID)

	kvClient, err := NewKvClient(kv.Authorizer)
	if err != nil {
		return false, err
	}

	signatureStr := base64.URLEncoding.EncodeToString(signature)
	digestStr := base64.URLEncoding.EncodeToString(digest)

	params := keyvault.KeyVerifyParameters{
		Algorithm: algorithm,
		Signature: &signatureStr,
		Digest:    &digestStr,
	}

	result, err := kvClient.Verify(ctx, kv.Name, keyID, keyVersion, params)
	if err != nil {
		return false, fmt.Errorf("failed to verify payload: %w", err)
	}
	return *result.Value, err

}
