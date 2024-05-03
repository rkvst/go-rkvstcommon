// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-common/tracing"
)

// Count counts the number of blobs filtered by the given tags filter
func (azp *Storer) Count(ctx context.Context, tagsFilter string, opts ...Option) (int64, error) {

	logger.Sugar.Debugf("Count")

	r, err := azp.FilteredList(ctx, tagsFilter, opts...)
	if err != nil {
		return 0, err
	}

	return int64(len(r.Items)), nil
}

type FilterResponse struct {
	Marker ListMarker // nil if no more pages

	// Standard request status things
	StatusCode int // For If- header fails, err can be nil and code can be 304
	Status     string

	Items []*azStorageBlob.FilterBlobItem
}

// FilteredList returns a list of blobs filtered on their tag values.
//
// tagsFilter examples:
//
//		 All tenants with more than one massif
//		     "firstindex">'0000000000000000'
//
//		 All tenants whose logs have been updated since a particular idtimestamp
//		     "lastid > '018e84dbbb6513a6'"
//
//	 note: in the case where you are making up the id timestamp from a time
//	 reading, set the least significant 24 bits to zero and use the hex encoding
//	 of the resulting value
//
//		All blobs in a storage account
//			"cat='tiger' AND penguin='emperorpenguin'"
//		All blobs in a specific container
//			"@container='zoo' AND cat='tiger' AND penguin='emperorpenguin'"
//
// See also: https://learn.microsoft.com/en-us/rest/api/storageservices/find-blobs-by-tags-container?tabs=microsoft-entra-id
//
// Returns all blobs with the specific tag filter.
func (azp *Storer) FilteredList(ctx context.Context, tagsFilter string, opts ...Option) (*FilterResponse, error) {
	span, ctx := tracing.StartSpanFromContext(ctx, "FilteredList")
	defer span.Finish()

	var err error

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.listMarker != nil {
		span.SetTag("marker", *options.listMarker)
	}
	o := &azStorageBlob.ServiceFilterBlobsOptions{
		Marker: options.listMarker,
		Where:  &tagsFilter,
	}

	if options.listMaxResults > 0 {
		o.MaxResults = &options.listMaxResults
		span.SetTag("maxResults", options.listMaxResults)
	}

	resp, err := azp.serviceClient.FindBlobsByTags(ctx, o)
	if err != nil {
		return nil, err
	}

	r := &FilterResponse{
		StatusCode: resp.RawResponse.StatusCode,
		Status:     resp.RawResponse.Status,
		Marker:     resp.NextMarker,
		Items:      resp.Blobs,
	}

	r.Marker = resp.NextMarker
	if r.Marker != nil {
		span.SetTag("nextmarker", *r.Marker)
	}

	return r, err
}

type ListerResponse struct {
	Marker ListMarker // nil if no more pages
	Prefix string

	// Standard request status things
	StatusCode int // For If- header fails, err can be nil and code can be 304
	Status     string

	Items []*azStorageBlob.BlobItemInternal
}

func (azp *Storer) List(ctx context.Context, opts ...Option) (*ListerResponse, error) {

	span, ctx := tracing.StartSpanFromContext(ctx, "ListBlobsFlat")
	defer span.Finish()

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.listMarker != nil {
		span.SetTag("marker", *options.listMarker)
	}
	o := azStorageBlob.ContainerListBlobsFlatOptions{
		Marker: options.listMarker,
	}
	if options.listPrefix != "" {
		o.Prefix = &options.listPrefix
		span.SetTag("prefix", options.listPrefix)
	}
	if options.listIncludeTags {
		o.Include = append(o.Include, azStorageBlob.ListBlobsIncludeItemTags)
	}
	if options.listIncludeMetadata {
		o.Include = append(o.Include, azStorageBlob.ListBlobsIncludeItemMetadata)
	}
	if options.listMaxResults > 0 {
		o.MaxResults = &options.listMaxResults
		span.SetTag("maxResults", options.listMaxResults)
	}

	// TODO: v1.21 feature which would be great
	// if options.listDelim != "" {
	// }
	r := &ListerResponse{}
	pager := azp.containerClient.ListBlobsFlat(&o)
	if !pager.NextPage(ctx) {
		return r, nil
	}
	resp := pager.PageResponse()
	r.Status = resp.RawResponse.Status
	r.StatusCode = resp.RawResponse.StatusCode

	if resp.Prefix != nil {
		r.Prefix = *resp.Prefix
	}

	r.Marker = resp.NextMarker
	if r.Marker != nil {
		span.SetTag("nextmarker", *r.Marker)
	}

	// Note: we pass on the azure type otherwise we would be copying for no good
	// reason. let the caller decided how to deal with that
	r.Items = resp.Segment.BlobItems

	return r, nil
}
