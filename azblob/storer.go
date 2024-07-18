// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"errors"
	"fmt"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

// so we dont have to import azure blob package anywhere else
type ContainerClient = azStorageBlob.ContainerClient
type ServiceClient = azStorageBlob.ServiceClient
type SharedKeyCredential = azStorageBlob.SharedKeyCredential

// Storer implements usage of Reader/Writer backed by azblob
type Storer struct {
	AccountName   string
	ResourceGroup string
	Subscription  string
	Container     string

	credential      *SharedKeyCredential
	rootURL         string
	containerURL    string
	containerClient *ContainerClient
	serviceClient   *ServiceClient

	log Logger
}

// New returns new az blob read/write object
func New(
	accountName string,
	resourceGroup string,
	subscription string,
	container string,
) (*Storer, error) {

	var err error
	logger.Sugar.Debugf("New Storer: %s/%s/%s/%s",
		accountName,
		resourceGroup,
		subscription,
		container,
	)

	secret, credential, err := credentials(
		accountName,
		resourceGroup,
		subscription,
	)
	if err != nil {
		return nil, err
	}
	rootURL := secret.URL

	if container == "" {
		msg := "storer: container is unspecified"
		logger.Sugar.Infof(msg)
		return nil, errors.New(msg)
	}
	azp := &Storer{
		AccountName:   accountName,
		ResourceGroup: resourceGroup,
		Subscription:  subscription,
		Container:     container,
		credential:    credential,
		rootURL:       rootURL,
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		rootURL,
		container,
	)
	azp.serviceClient, err = azStorageBlob.NewServiceClientWithSharedKey(
		rootURL,
		credential,
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
