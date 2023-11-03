package azblob

import (
	"errors"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

func storerOptionConditions(options *StorerOptions) (azStorageBlob.BlobAccessConditions, error) {

	var blobAccessConditions azStorageBlob.BlobAccessConditions
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
	return blobAccessConditions, nil
}
