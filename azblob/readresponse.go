package azblob

import (
	"errors"
	"io"
	"net/textproto"
	"strconv"
	"time"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

type ReadResponseScannedStatus func(resp *ReaderResponse, metaData map[string]string)

type ReaderResponse struct {
	Reader            io.ReadCloser
	HashValue         string
	MimeType          string
	ContentLength     int64
	Size              int64 // MIME size
	Tags              map[string]string
	TimestampAccepted string
	ScannedStatus     string
	ScannedBadReason  string
	ScannedTimestamp  string

	BlobClient *azStorageBlob.BlobClient

	// The following are copied as appropriate from the azure sdk response.
	// See also WriterResponse
	ETag         *string
	LastModified *time.Time
	Metadata     map[string]string // x-ms-meta header
	StatusCode   int               // For If- header fails, err can be nil and code can be 304
	Status       string
	XMsErrorCode string // will be "ConditioNotMet" for If- header predicate fails, even when err is nil

	setReadResponseScannedStatus ReadResponseScannedStatus
}

func (r *ReaderResponse) ConditionNotMet() bool {
	return r.XMsErrorCode == string(azStorageBlob.StorageErrorCodeConditionNotMet)
}

// Ok returns true if the http status was 200 or 201
// This method is provided for use in combination with specific headers like
// If-Match and ETags conditions.  In thos circumstances we often get err=nil
// but no content.
func (r *ReaderResponse) Ok() bool {
	return r.StatusCode == 200 || r.StatusCode == 201
}

const (
	xMsErrorCodeHeader = "X-Ms-Error-Code"
)

// normaliseReaderResponseErr propagates appropriate err details to the response
// this makes it easier to do consistent checking of responses when using ETags
// and other conditional header features.
//
// Does nothing unless err can be handled As(azure-sdk.StorageError)
func normaliseReaderResponseErr(err error, rr *ReaderResponse) {
	if err == nil {
		return
	}

	var terr *azStorageBlob.StorageError
	if !errors.As(err, &terr) {
		return
	}
	if terr.ErrorCode != "" {
		rr.XMsErrorCode = string(terr.ErrorCode)
		switch terr.ErrorCode {
		case azStorageBlob.StorageErrorCodeConditionNotMet:
			rr.Status = "304 " + string(terr.ErrorCode)
			rr.StatusCode = 304
		default:
		}
	}
}

// downloadReaderResponse copies accross the azure sdk response values that are
// meaningful to our supported api
func downloadReaderResponse(r azStorageBlob.BlobDownloadResponse, rr *ReaderResponse) error {
	rr.Status = r.RawResponse.Status
	rr.StatusCode = r.RawResponse.StatusCode

	rr.LastModified = r.LastModified
	rr.ETag = r.ETag
	rr.Metadata = r.Metadata

	value, ok := r.RawResponse.Header[xMsErrorCodeHeader]
	if ok && len(value) > 0 {
		rr.XMsErrorCode = value[0]
	}

	s, ok := r.RawResponse.Header["Content-Length"]
	if !ok {
		return nil
	}
	if len(s) == 0 {
		rr.ContentLength = 0
		return nil
	}

	var err error
	rr.ContentLength, err = strconv.ParseInt(s[0], 10, 64)
	return err
}

// readerResponseMetadata processes and conditions values from the metadata we have specific support for.
func readerResponseMetadata(resp *ReaderResponse, metaData map[string]string) error {
	size, parseErr := strconv.ParseInt(metaData[textproto.CanonicalMIMEHeaderKey(SizeKey)], 10, 64)
	if parseErr != nil {
		logger.Sugar.Infof("cannot get size value: %v", parseErr)
		return parseErr
	}
	resp.Size = size
	resp.HashValue = metaData[textproto.CanonicalMIMEHeaderKey(HashKey)]
	resp.MimeType = metaData[textproto.CanonicalMIMEHeaderKey(MimeKey)]
	resp.TimestampAccepted = metaData[textproto.CanonicalMIMEHeaderKey(TimeKey)]
	if resp.setReadResponseScannedStatus != nil {
		resp.setReadResponseScannedStatus(resp, metaData)
	}
	// Note: it is fine if these are the same instances
	resp.Metadata = metaData
	return nil
}
