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
// NOTE: due to having no credentials, this can only read from public blob storage.
//
// example:
//
//	accountName: jitavid3b5fc07b9ae06f4e
//	url: https://jitavid3b5fc07b9ae06f4e.blob.core.windows.net
//	container: merklebuilder
func NewReaderNoAuth(accountName string, url string, container string) (Reader, error) {
	logger.Sugar.Infof(
		"New Reader with accountName: %s, for container: %s",
		accountName, container,
	)

	var err error

	if accountName == "" || url == "" {
		return nil, errors.New("missing connection configuration variables")
	}

	azp := &Storer{
		AccountName:   accountName,
		ResourceGroup: "", // just for logging
		Subscription:  "", // just for logging
		Container:     container,
		credential:    nil,
		rootURL:       url,
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		url,
		container,
	)
	azp.serviceClient, err = azStorageBlob.NewServiceClientWithNoCredential(
		url,
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
