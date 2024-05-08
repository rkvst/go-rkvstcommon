package azblob

import (
	"context"
	"errors"
	"fmt"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

/**
 * Reader is an azblob reader, with read only methods.
 */

// Reader is the interface in order to carry out read operations on the azure blob storage
type Reader interface {
	Reader(
		ctx context.Context,
		identity string,
		opts ...Option,
	) (*ReaderResponse, error)
	FilteredList(ctx context.Context, tagsFilter string, opts ...Option) (*FilterResponse, error)
	List(ctx context.Context, opts ...Option) (*ListerResponse, error)
}

// NewReaderNoAuth is a azure blob reader client that has no credentials.
//
// The provided accountName is only used for logging purposes and may be empty
// The pro
//
// Paramaters:
//
//	accountName: used only for logging purposes and may be empty
//	url: The root path for the blob store requests, must not be empty
//	container: To use the container client this must be provided. If absent only storage account level apis can be used
//
// NOTE: due to having no credentials, this can only read from public blob storage.
// or proxied private blob storage.
//
// example:
//
//	accountName: jitavid3b5fc07b9ae06f4e
//	url: https://jitavid3b5fc07b9ae06f4e.blob.core.windows.net
//	container: merklebuilder
func NewReaderNoAuth(accountName string, url string, container string) (Reader, error) {
	logger.Sugar.Infof(
		"New Reader for url: %s, with accountName: %s, for container: %s",
		url, accountName, container,
	)

	var err error
	if url == "" {
		return nil, errors.New("url is a required parameter and cannot be empty")
	}

	azp := &Storer{
		AccountName:   accountName, // just for logging
		ResourceGroup: "",          // just for logging
		Subscription:  "",          // just for logging
		Container:     container,
		credential:    nil,
		rootURL:       url,
	}
	azp.serviceClient, err = azStorageBlob.NewServiceClientWithNoCredential(
		url,
		nil,
	)
	if err != nil {
		logger.Sugar.Infof("unable to create serviceclient %s: %v", url, err)
		return nil, err
	}

	if container == "" {
		logger.Sugar.Infof("container not provided, container client not created")
		return nil, nil
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		url,
		container,
	)
	azp.containerClient, err = azp.serviceClient.NewContainerClient(container)
	if err != nil {
		logger.Sugar.Infof("unable to create containerclient %s: %v", container, err)
		return nil, err
	}

	return azp, nil
}
