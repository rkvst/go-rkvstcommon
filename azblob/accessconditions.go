package azblob

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"
)

func storerOptionConditions(options *StorerOptions) (blob.AccessConditions, error) {

	var blobAccessConditions blob.AccessConditions
	if options.leaseID == "" && options.etagCondition == EtagNotUsed {
		return blobAccessConditions, nil
	}
	if options.etag == "" && options.etagCondition != EtagNotUsed {
		return blobAccessConditions, errors.New("etag value missing")
	}

	blobAccessConditions = blob.AccessConditions{}
	if options.leaseID != "" {
		blobAccessConditions.LeaseAccessConditions = &lease.AccessConditions{
			LeaseID: &options.leaseID,
		}
	}

	blobAccessConditions.ModifiedAccessConditions = &blob.ModifiedAccessConditions{}

	switch options.etagCondition {
	case ETagMatch:
		t := azcore.ETag(options.etag)
		blobAccessConditions.ModifiedAccessConditions.IfMatch = &t
	case ETagNoneMatch:
		t := azcore.ETag(options.etag)
		blobAccessConditions.ModifiedAccessConditions.IfNoneMatch = &t
	case TagsWhere:
		blobAccessConditions.ModifiedAccessConditions.IfTags = &options.etag
	default:
	}
	switch options.sinceCondition {
	case IfConditionModifiedSince:
		blobAccessConditions.ModifiedAccessConditions.IfModifiedSince = options.since
	case IfConditionUnmodifiedSince:
		blobAccessConditions.ModifiedAccessConditions.IfUnmodifiedSince = options.since
	default:
	}
	return blobAccessConditions, nil
}
