// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

const (
	// metadata keys
	ContentKey = "content_type"
	HashKey    = "hash"
	MimeKey    = "mime_type"
	SizeKey    = "size"
	TimeKey    = "time_accepted"
)

// getTags gets tags from blob storage
func (azp *Storer) getTags(
	ctx context.Context,
	identity string,
) (map[string]string, error) {

	var err error

	blobClient, err := azp.containerClient.NewBlobClient(identity)
	if err != nil {
		return nil, ErrorFromError(err)
	}

	resp, err := blobClient.GetTags(ctx, nil)
	if err != nil {
		return nil, ErrorFromError(err)
	}
	tags := make(map[string]string, len(resp.BlobTagSet))
	for _, tag := range resp.BlobTagSet {
		tags[*tag.Key] = *tag.Value
	}
	return tags, nil
}

// getMetadata gets metadata from blob storage
func (azp *Storer) getMetadata(
	ctx context.Context,
	identity string,
) (map[string]string, error) {

	blobClient, err := azp.containerClient.NewBlobClient(identity)
	if err != nil {
		return nil, ErrorFromError(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, ErrorFromError(err)
	}
	return resp.Metadata, nil
}

// Reader creates a reader.
func (azp *Storer) Reader(
	ctx context.Context,
	identity string,
	opts ...Option,
) (*ReaderResponse, error) {

	var err error

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	resp := &ReaderResponse{setReadResponseScannedStatus: azp.setReadResponseScannedStatus}
	blobAccessConditions, err := storerOptionConditions(options)
	if err != nil {
		return nil, err
	}

	if len(options.tags) > 0 || options.getTags {
		tags, tagsErr := azp.getTags(
			ctx,
			identity,
		)
		if tagsErr != nil {
			return nil, tagsErr
		}
		resp.Tags = tags
	}

	// XXX: TODO this should be done with access conditions. this is racy as it
	// stands. azure guarantees the tags for a blob read after write is
	// consistent. we can't take advantage of that while this remains racy.
	for k, requiredValue := range options.tags {
		blobValue, ok := resp.Tags[k]
		if !ok {
			return nil, NewStatusError(fmt.Sprintf("tag %s is not specified on blob", k), http.StatusNotFound)
		}
		if blobValue != requiredValue {
			return nil, NewStatusError(fmt.Sprintf("blob has different Tag %s than required %s", blobValue, requiredValue), http.StatusNotFound)
		}
	}

	// If we are *only* getting metadata, issue a distinct request. Otherwise we
	// get it from the download response.
	if options.getMetadata == OnlyMetadata {
		metaData, metadataErr := azp.getMetadata(
			ctx,
			identity,
		)
		if metadataErr != nil {
			return nil, metadataErr
		}
		if parseErr := readerResponseMetadata(resp, metaData); parseErr != nil {
			return nil, err
		}
	}

	if options.getMetadata == OnlyMetadata {
		return resp, nil
	}

	// check if there is a container client
	//  as the storer may just have a service client
	if azp.containerClient == nil {
		return nil, errors.New("no container client available for reader")
	}

	resp.BlobClient, err = azp.containerClient.NewBlobClient(identity)
	if err != nil {
		return nil, ErrorFromError(err)
	}
	countToEnd := int64(azStorageBlob.CountToEnd)
	get, err := resp.BlobClient.Download(
		ctx,
		&azStorageBlob.BlobDownloadOptions{
			BlobAccessConditions: &blobAccessConditions,
			Count:                &countToEnd,
		},
	)

	if err != nil && err == io.EOF { // nolint
		return nil, ErrorFromError(err)
	}

	normaliseReaderResponseErr(err, resp)
	if err == nil {
		// We *always* copy the metadata into the response
		err = downloadReaderResponse(get, resp)
		if err != nil {
			return resp, err
		}

		// for backwards compat, we only process the metadata on request
		if options.getMetadata == BothMetadataAndBlob {
			_ = readerResponseMetadata(resp, resp.Metadata) // the parse error is benign
		}
	}

	if get.RawResponse != nil {
		resp.Reader = get.Body(nil)
	}
	return resp, err
}

func (r *ReaderResponse) DownloadToWriter(w io.Writer) error {
	defer r.Reader.Close()
	_, err := io.Copy(w, r.Reader)
	return err
}
