package azblob

import (
	"errors"
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
	StatusCode   int // For If- header fails, err can be nil and code can be 304
	Status       string
	XMsErrorCode string // will be "ConditioNotMet" for If- header predicate fails, even when err is nil
}

// normaliseWriteResponseErr propagates appropriate err details to the response
// this makes it easier to do consistent checking of responses when using ETags
// and other conditional header features.
//
// Does nothing unless err can be handled As(azure-sdk.StorageError)
func normaliseWriteResponseErr(err error, wr *WriteResponse) {
	if err == nil {
		return
	}

	var terr *azStorageBlob.StorageError
	if !errors.As(err, &terr) {
		return
	}
	if terr.ErrorCode != "" {
		wr.XMsErrorCode = string(terr.ErrorCode)
		switch terr.ErrorCode {
		case azStorageBlob.StorageErrorCodeConditionNotMet:
			wr.Status = "304 " + string(terr.ErrorCode)
			wr.StatusCode = 304
		default:
		}
	}
}

func uploadStreamWriteResponse(r azStorageBlob.BlockBlobCommitBlockListResponse) *WriteResponse {
	w := WriteResponse{
		ETag: r.ETag,
	}
	w.Status = r.RawResponse.Status
	w.StatusCode = r.RawResponse.StatusCode
	value, ok := r.RawResponse.Header[xMsErrorCodeHeader]
	if ok && len(value) > 0 {
		w.XMsErrorCode = value[0]
	}

	return &w
}

func uploadWriteResponse(r azStorageBlob.BlockBlobUploadResponse) *WriteResponse {
	w := WriteResponse{
		ETag: r.ETag,
	}
	w.Status = r.RawResponse.Status
	w.StatusCode = r.RawResponse.StatusCode
	value, ok := r.RawResponse.Header[xMsErrorCodeHeader]
	if ok && len(value) > 0 {
		w.XMsErrorCode = value[0]
	}

	return &w
}
