// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"

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
func (azp *Storer) FilteredList(ctx context.Context, tagsFilter string) ([]*service.FilterBlobItem, error) {
	logger.Sugar.Debugf("FilteredList")

	var filteredBlobs []*service.FilterBlobItem
	var err error

	result, err := azp.serviceClient.FilterBlobs(
		ctx,
		tagsFilter,
		nil,
	)
	if err != nil {
		return nil, err
	}

	filteredBlobs = result.FilterBlobSegment.Blobs

	return filteredBlobs, err
}

type ListerResponse struct {
	Marker ListMarker // nil if no more pages
	Prefix string

	Items []*container.BlobItem
}

func (azp *Storer) List(ctx context.Context, opts ...Option) (*ListerResponse, error) {

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	o := azStorageBlob.ListBlobsFlatOptions{
		Marker: options.listMarker,
		Include: container.ListBlobsInclude{
			Metadata: options.listIncludeMetadata,
			Tags:     options.listIncludeTags,
		},
	}
	if options.listPrefix != "" {
		o.Prefix = &options.listPrefix
	}

	// TODO: v1.21 feature which would be great
	// if options.listDelim != "" {
	// }
	r := &ListerResponse{Items: []*container.BlobItem{}}

	// blob listings are returned across multiple pages
	pager := azp.containerClient.NewListBlobsFlatPager(&o)
	resp, err := pager.NextPage(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Prefix != nil {
		r.Prefix = *resp.Prefix
	}
	r.Items = append(r.Items, resp.Segment.BlobItems...)
	// continue fetching pages until no more remain
	for pager.More() {
		// advance to the next page
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if resp.Prefix != nil {
			r.Prefix = *resp.Prefix
		}
		// Note: we pass on the azure type otherwise we would be copying for no good
		// reason. let the caller decided how to deal with that
		r.Items = append(r.Items, resp.Segment.BlobItems...)
	}
	return r, nil
}
