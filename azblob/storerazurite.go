package azblob

import (
	"errors"
	"fmt"
	"os"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

type DevConfig struct {
	AccountName string
	Key         string
	URL         string
}

const (
	// These constants are well known and described here:
	// See: https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite

	azureStorageAccountVar    string = "AZURE_STORAGE_ACCOUNT"
	azureStorageKeyVar        string = "AZURE_STORAGE_KEY"
	azuriteBlobEndpointURLVar string = "AZURITE_BLOB_ENDPOINT_URL"

	azuriteWellKnownKey             string = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	azuriteWellKnownAccount         string = "devstoreaccount1"
	azuriteWellKnownBlobEndpointURL string = "http://127.0.0.1:10000/devstoreaccount1/"
	azuriteResourceGroup            string = "azurite-emulator"
	azuriteSubscription             string = "azurite-emulator"
)

// NewDevConfigFromEnv reads azurite (azure emulator) config from the standard
// azure env vars and falls back to the docmented defaults if they are not set.
// If overriding any settings via env, be sure to also configure
// AZURITE_ACCOUNTS for the emulator
// See: https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite
func NewDevConfigFromEnv() DevConfig {

	// This is not for production, it is specifically for testing, hence the use
	// of programed in defaults.
	return DevConfig{
		AccountName: devVarWithDefault(azureStorageAccountVar, azuriteWellKnownAccount),
		Key:         devVarWithDefault(azureStorageKeyVar, azuriteWellKnownKey),
		URL:         devVarWithDefault(azuriteBlobEndpointURLVar, azuriteWellKnownBlobEndpointURL),
	}
}

// GetContainerClient returns the underlying container client
func (s *Storer) GetContainerClient() *azStorageBlob.ContainerClient {
	return s.containerClient
}

// GetServiceClient returns the underlying service client
func (s *Storer) GetServiceClient() *azStorageBlob.ServiceClient {
	return s.serviceClient
}

// NewDev returns a normal blob client but connected for the azurite local
// emulator It uses the well known account name and key by default. If
// overriding, be sure to also configure AZURITE_ACCOUNTS for the emulator
// See: https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite
func NewDev(cfg DevConfig, container string) (*Storer, error) {
	logger.Sugar.Infof(
		"Attempt environment auth with accountName: %s, for container: %s",
		cfg.AccountName, container,
	)

	if cfg.AccountName == "" || cfg.Key == "" || cfg.URL == "" {
		return nil, errors.New("missing connection configuration variables")
	}
	cred, err := azStorageBlob.NewSharedKeyCredential(cfg.AccountName, cfg.Key)
	if err != nil {
		return nil, err
	}

	azp := &Storer{
		AccountName:   cfg.AccountName,
		ResourceGroup: azuriteResourceGroup, // just for logging
		Subscription:  azuriteSubscription,  // just for logging
		Container:     container,
		credential:    cred,
		rootURL:       cfg.URL,
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		cfg.URL,
		container,
	)
	azp.serviceClient, err = azStorageBlob.NewServiceClientWithSharedKey(
		cfg.URL,
		cred,
		nil,
	)
	if err != nil {
		logger.Sugar.Infof("unable to create serviceclient %s: %v", azp.containerURL, err)
		return nil, err
	}
	azp.containerClient, err = azp.serviceClient.NewContainerClient(container)
	if err != nil {
		logger.Sugar.Infof("unable to create containerclient %s: %v", container, err)
		return nil, err
	}

	return azp, nil
}

// devVarWithDefault reads the key from env.
// If key is not set, returns  the defaultValue.
func devVarWithDefault(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
