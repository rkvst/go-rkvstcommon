package azblob

import (
	"context"
	"errors"
	"fmt"

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
// The provided accountName is only used for logging purposes and may be empty
// The pro
//
// Paramaters:
//
//	accountName: used only for logging purposes and may be empty
//	url: The root path for the blob store requests, must not be empty
//	opts: optional arguments specific to creating a reader with no auth
//
//	  * WithAccountName() - specifies the azblob account name for logging
//
// NOTE: due to having no credentials, this can only read from public blob storage.
// or proxied private blob storage.
//
// example:
//
//	url: https://app.datatrails.ai/verifiabledata
//	container: merklelogs
func NewReaderNoAuth(url string, container string, opts ...ReaderOption) (Reader, error) {

	var err error
	if url == "" || container == "" {
		return nil, errors.New("url and container are required parameters and neither can be empty")
	}

	readerOptions := ParseReaderOptions(opts...)

	azp := &Storer{
		AccountName:   readerOptions.accountName, // just for logging
		ResourceGroup: "",                        // just for logging
		Subscription:  "",                        // just for logging
		Container:     container,
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

	azp.containerURL = fmt.Sprintf(
		"%s%s",
		url,
		container,
	)
	azp.containerClient, err = azp.serviceClient.NewContainerClient(container)
	if err != nil {
		return nil, err
	}

	return azp, nil
}
