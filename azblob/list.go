// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	"github.com/rkvst/go-rkvstcommon/logger"
)

// Count counts the number of blobs filtered by the given tags filter
func (azp *Storer) Count(ctx context.Context, tagsFilter string) (int64, error) {

	logger.Sugar.Debugf("Count")

	blobs, err := azp.FilteredList(ctx, tagsFilter)
	if err != nil {
		return 0, err
	}

	return int64(len(blobs)), nil
}

// FilteredList returns a list of blobs filtered on their tag values.
//
// tagsFilter example: "dog='germanshepherd' and penguin='emperorpenguin'"
// Returns all blobs with the specific tag filter
func (azp *Storer) FilteredList(ctx context.Context, tagsFilter string) ([]*azStorageBlob.FilterBlobItem, error) {
	logger.Sugar.Debugf("FilteredList")

	var filteredBlobs []*azStorageBlob.FilterBlobItem
	var err error

	result, err := azp.serviceClient.FindBlobsByTags(
		ctx,
		&azStorageBlob.ServiceFilterBlobsOptions{
			Where: &tagsFilter,
		},
	)
	if err != nil {
		return filteredBlobs, err
	}

	filteredBlobs = result.Blobs

	return filteredBlobs, err
}
