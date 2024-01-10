package azblob

import (
	"errors"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

func storerOptionConditions(options *StorerOptions) (blob.AccessConditions, error) {

	var blobAccessConditions blob.AccessConditions
	if options.leaseID == "" && options.etagCondition == EtagNotUsed {
		return blobAccessConditions, nil
	}
	if options.etag == "" && options.etagCondition != EtagNotUsed {
		return blobAccessConditions, errors.New("etag value missing")
	}

	blobAccessConditions = azStorageBlob.BlobAccessConditions{}
	if options.leaseID != "" {
		blobAccessConditions.LeaseAccessConditions = &azStorageBlob.LeaseAccessConditions{
			LeaseID: &options.leaseID,
		}
	}

	blobAccessConditions.ModifiedAccessConditions = &azStorageBlob.ModifiedAccessConditions{}

	switch options.etagCondition {
	case ETagMatch:
		blobAccessConditions.ModifiedAccessConditions.IfMatch = &options.etag
	case ETagNoneMatch:
		blobAccessConditions.ModifiedAccessConditions.IfNoneMatch = &options.etag
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
