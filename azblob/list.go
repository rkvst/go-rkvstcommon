// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"

	"github.com/datatrails/go-datatrails-common/logger"
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

type ListerResponse struct {
	Marker ListMarker // nil if no more pages
	Prefix string

	// Standard request status things
	StatusCode int // For If- header fails, err can be nil and code can be 304
	Status     string

	Items []*container.BlobItem
}

func (azp *Storer) List(ctx context.Context, opts ...Option) (*ListerResponse, error) {

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	o := azStorageBlob.ContainerListBlobsFlatOptions{
		Marker: options.listMarker,
	}
	if options.listPrefix != "" {
		o.Prefix = &options.listPrefix
	}
	if options.listIncludeTags {
		o.Include = append(o.Include, azStorageBlob.ListBlobsIncludeItemTags)
	}
	if options.listIncludeMetadata {
		o.Include = append(o.Include, azStorageBlob.ListBlobsIncludeItemMetadata)
	}

	// TODO: v1.21 feature which would be great
	// if options.listDelim != "" {
	// }
	r := &ListerResponse{Items: []*container.BlobItem{}}

	// blob listings are returned across multiple pages
	pager := azp.containerClient.NewListBlobsFlatPager(azp.container, &o)
	if !pager.NextPage(ctx) {
		return r, nil
	}
	resp := pager.PageResponse()
	r.Status = resp.RawResponse.Status
	r.StatusCode = resp.RawResponse.StatusCode
	if resp.Prefix != nil {
		r.Prefix = *resp.Prefix
	}

	// continue fetching pages until no more remain
	for pager.More() {
		// advance to the next page
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		// Note: we pass on the azure type otherwise we would be copying for no good
		// reason. let the caller decided how to deal with that
		r.Items = append(r.Items, resp.Segment.BlobItems...)
	}
	return r, nil
}
