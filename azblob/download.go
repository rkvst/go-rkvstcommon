// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"
	"fmt"
	"io"
	"net/http"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	"github.com/datatrails/go-datatrails-common/logger"
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
	logger.Sugar.Debugf("getTags BlockBlob URL %s", identity)

	blobClient, err := azp.containerClient.NewBlobClient(identity)
	if err != nil {
		logger.Sugar.Debugf("getTags BlockBlob Client %s error: %v", identity, err)
		return nil, ErrorFromError(err)
	}

	resp, err := blobClient.GetTags(ctx, nil)
	if err != nil {
		logger.Sugar.Debugf("getTags BlockBlob URL %s error: %v", identity, err)
		return nil, ErrorFromError(err)
	}
	logger.Sugar.Debugf("getTags BlockBlob tagSet: %v", resp.BlobTagSet)
	tags := make(map[string]string, len(resp.BlobTagSet))
	for _, tag := range resp.BlobTagSet {
		tags[*tag.Key] = *tag.Value
	}
	logger.Sugar.Debugf("getTags BlockBlob URL %s tags: %v", identity, tags)
	return tags, nil
}

// getMetadata gets metadata from blob storage
func (azp *Storer) getMetadata(
	ctx context.Context,
	identity string,
) (map[string]string, error) {
	logger.Sugar.Debugf("getMetadata BlockBlob URL %s", identity)

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
	logger.Sugar.Debugf("getMetadata BlockBlob URL %v", resp.Metadata)
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

	logger.Sugar.Debugf("Reader BlockBlob URL %s", identity)

	resp := &ReaderResponse{}
	blobAccessConditions, err := storerOptionConditions(options)
	if err != nil {
		return nil, err
	}

	if len(options.tags) > 0 || options.getTags {
		logger.Sugar.Debugf("Get tags")
		tags, tagsErr := azp.getTags(
			ctx,
			identity,
		)
		if tagsErr != nil {
			logger.Sugar.Infof("cannot get tags: %v", tagsErr)
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
			logger.Sugar.Infof("tag %s is not specified on blob", k)
			return nil, NewStatusError(fmt.Sprintf("tag %s is not specified on blob", k), http.StatusNotFound)
		}
		if blobValue != requiredValue {
			logger.Sugar.Infof("blob has different Tag %s than required %s", blobValue, requiredValue)
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
			logger.Sugar.Infof("cannot get metadata: %v", metadataErr)
			return nil, metadataErr
		}
		if parseErr := readerResponseMetadata(resp, metaData); parseErr != nil {
			return nil, err
		}
	}

	if options.getMetadata == OnlyMetadata {
		return resp, nil
	}

	logger.Sugar.Debugf("Creating New io.Reader")
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
		logger.Sugar.Infof("cannot get blob body: %v", err)
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
