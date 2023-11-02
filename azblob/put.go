package azblob

import (
	"context"
	"fmt"
	"io"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/rkvst/go-rkvstcommon/logger"
)

// Put creates or replaces a blob
// metadata and tags are set in the same operation as the content update.
func (azp *Storer) Put(
	ctx context.Context,
	identity string,
	source io.ReadSeekCloser,
	opts ...Option,
) (*WriteResponse, error) {
	err := azp.checkContainer(ctx)
	if err != nil {
		return nil, err
	}
	logger.Sugar.Debugf("Create or replace BlockBlob %s", identity)

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	wr, err := azp.putBlob(
		ctx, identity, source, options)
	if err != nil {
		return nil, err
	}
	return wr, nil
}

// putBlob creates or replaces a blob. If the blob exists, any existing metdata
// is replaced in its entirity. It is an error if the seek position of the
// reader can't be set to zero
// ref: https://learn.microsoft.com/en-gb/rest/api/storageservices/put-blob?tabs=azure-ad
func (azp *Storer) putBlob(
	ctx context.Context,
	identity string,
	body io.ReadSeekCloser,
	options *StorerOptions,
) (*WriteResponse, error) {
	logger.Sugar.Debugf("write %s", identity)

	// The az sdk panics if this is not the case, we want an err
	if pos, err := body.Seek(0, io.SeekCurrent); pos != 0 || err != nil {
		return nil, fmt.Errorf("bad body for %s: %v", identity, ErrMustSupportSeek0)
	}

	blobAccessConditions, err := storerOptionConditions(options)
	if err != nil {
		return nil, err
	}

	blockBlobClient, err := azp.containerClient.NewBlockBlobClient(identity)
	if err != nil {
		logger.Sugar.Infof("Cannot get block blob client blob: %v", err)
		return nil, ErrorFromError(err)
	}

	r, err := blockBlobClient.Upload(
		ctx,
		body,
		&azStorageBlob.BlockBlobUploadOptions{
			BlobAccessConditions: &blobAccessConditions,
			Metadata:             options.metadata,
			TagsMap:              options.tags,
		},
	)
	if err != nil {
		logger.Sugar.Infof("Cannot upload blob: %v", err)
		return nil, ErrorFromError(err)
	}
	return uploadWriteResponse(r), nil
}
