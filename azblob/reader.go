package azblob

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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
// Paramaters:
//
//		accountName: used only for logging purposes and may be empty
//		url: The root path for the blob store requests, must not be empty
//		opts: optional arguments specific to creating a reader with no auth
//
//	   * WithContainer() - specifies the azblob container for point get in the container
//	   * WithAccountName() - specifies the azblob account name for logging
//
// NOTE: due to having no credentials, this can only read from public blob storage.
// or proxied private blob storage.
//
// NOTE: if no optional container is specified than the Reader() method on the interface
// will error, as we cannot create a container client reader.
//
// example:
//
//	url: https://app.datatrails.ai/verifiabledata
func NewReaderNoAuth(url string, opts ...ReaderOption) (Reader, error) {
	var err error
	if url == "" {
		return nil, errors.New("url is a required parameter and cannot be empty")
	}

	readerOptions := ParseReaderOptions(opts...)

	azp := &Storer{
		AccountName:   readerOptions.accountName, // just for logging
		ResourceGroup: "",                        // just for logging
		Subscription:  "",                        // just for logging
		Container:     readerOptions.container,
		credential:    nil,
		rootURL:       url,
	}
	azp.serviceClient, err = azStorageBlob.NewServiceClientWithNoCredential(
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// check if we need the container client as well,
	//  if the container is an empty string just return the storer with
	//  the service client
	//
	// NOTE: Only listing is available to the serviceClient
	if readerOptions.container == "" {
		return azp, nil
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		url,
		readerOptions.container,
	)
	azp.containerClient, err = azp.serviceClient.NewContainerClient(readerOptions.container)
	if err != nil {
		return nil, err
	}

	return azp, nil
}

// NewReaderDefaultAuth is a azure blob reader client that obtains credentials from the
// environment - including aad pod identity / workload identity.
func NewReaderDefaultAuth(url string, opts ...ReaderOption) (Reader, error) {
	var err error
	if url == "" {
		return nil, errors.New("url is a required parameter and cannot be empty")
	}

	readerOptions := ParseReaderOptions(opts...)

	azp := &Storer{
		AccountName:   readerOptions.accountName, // just for logging
		ResourceGroup: "",                        // just for logging
		Subscription:  "",                        // just for logging
		Container:     readerOptions.container,
		credential:    nil,
		rootURL:       url,
	}

	credentials, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	azp.serviceClient, err = azStorageBlob.NewServiceClient(
		url,
		credentials,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// check if we need the container client as well,
	//  if the container is an empty string just return the storer with
	//  the service client
	//
	// NOTE: Only listing is available to the serviceClient
	if readerOptions.container == "" {
		return azp, nil
	}

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		url,
		readerOptions.container,
	)
	azp.containerClient, err = azp.serviceClient.NewContainerClient(readerOptions.container)
	if err != nil {
		return nil, err
	}

	return azp, nil
}
