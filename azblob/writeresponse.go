package azblob

import (
	"time"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type WriteResponse struct {
	HashValue         string
	MimeType          string
	Size              int64
	TimestampAccepted string

	// The following fields are copied from the sdk response. nil pointers mean
	// 'not set'.  Note that different azure blob sdk write mode apis (put,
	// create, stream) etc return different generated types. So we copy only the
	// fields which make sense into this unified type. And we only copy values
	// which can be made use of in the api presented by this package.
	ETag         *string
	LastModified *time.Time
}

func uploadStreamWriteResponse(r azStorageBlob.BlockBlobCommitBlockListResponse) *WriteResponse {
	w := WriteResponse{
		ETag: r.ETag,
	}
	return &w
}

func uploadWriteResponse(r azStorageBlob.BlockBlobUploadResponse) *WriteResponse {
	w := WriteResponse{
		ETag: r.ETag,
	}
	return &w
}
