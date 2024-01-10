// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"time"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	mimetype "github.com/gabriel-vasile/mimetype"

	"github.com/datatrails/go-datatrails-common/logger"
)

var (
	ErrMustSupportSeek0 = errors.New("must be seekable to position 0")
)

const (
	chunkSize = 2 * 1024 * 1024
)

func (azp *Storer) checkContainer(ctx context.Context) error {
	logger.Sugar.Debugf("Checking container URL %s", azp.containerURL)
	_, err := azp.containerClient.GetProperties(ctx, nil)
	if err != nil {
		return ErrorFromError(err)
	}
	return nil
}

// setMetadata sets metadata in blob storage
func (azp *Storer) setMetadata(
	ctx context.Context,
	identity string,
	metadata map[string]string,
) error {
	logger.Sugar.Debugf("setMetadata BlockBlob %s: %v", identity, metadata)

	blobClient, err := azp.containerClient.NewBlobClient(identity)
	if err != nil {
		return ErrorFromError(err)
	}
	_, err = blobClient.SetMetadata(ctx, metadata, nil)
	if err != nil {
		return ErrorFromError(err)
	}
	return nil
}

// setTags sets Tags in blob storage
func (azp *Storer) setTags(
	ctx context.Context,
	identity string,
	tags map[string]string,
) error {
	logger.Sugar.Debugf("setTags BlockBlob %s: %v", identity, tags)

	blobClient, err := azp.containerClient.NewBlobClient(identity)
	if err != nil {
		return ErrorFromError(err)
	}
	_, err = blobClient.SetTags(
		ctx,
		&azStorageBlob.BlobSetTagsOptions{
			TagsMap: tags,
		},
	)
	if err != nil {
		return ErrorFromError(err)
	}
	return nil
}

// Write writes to blob from io.Reader.
func (azp *Storer) Write(
	ctx context.Context,
	identity string,
	source io.Reader,
	opts ...Option,
) (*WriteResponse, error) {
	err := azp.checkContainer(ctx)
	if err != nil {
		return nil, err
	}
	logger.Sugar.Debugf("Create BlockBlob %s", identity)

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.etagCondition != EtagNotUsed {
		return nil, errors.New("etag conditions are not supported on streaming uploads")
	}

	wr, err := azp.writeStream(ctx, identity, source, options.leaseID)
	if err != nil {
		return nil, err
	}
	if options.metadata != nil {
		// upload metadata
		err = azp.setMetadata(ctx, identity, options.metadata)
		if err != nil {
			return nil, err
		}
	}
	if options.tags != nil {
		// upload tags
		err = azp.setTags(ctx, identity, options.tags)
		if err != nil {
			return nil, err
		}
	}
	return wr, nil
}

// Write writes to blob from http request.
func (azp *Storer) WriteStream(
	ctx context.Context,
	identity string,
	source *http.Request,
	opts ...Option,
) (*WriteResponse, error) {
	err := azp.checkContainer(ctx)
	if err != nil {
		return nil, err
	}
	logger.Sugar.Debugf("Create BlockBlob %s", identity)

	options := &StorerOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return azp.streamReader(ctx, identity, source, options)
}

func (azp *Storer) writeStream(
	ctx context.Context,
	identity string,
	reader io.Reader,
	leaseID string,
) (*WriteResponse, error) {
	logger.Sugar.Debugf("write %s", identity)
	blockBlobClient, err := azp.containerClient.NewBlockBlobClient(identity)
	if err != nil {
		logger.Sugar.Infof("Cannot get block blob client blob: %v", err)
		return nil, ErrorFromError(err)
	}
	blobAccessConditions := azStorageBlob.BlobAccessConditions{
		LeaseAccessConditions:    &azStorageBlob.LeaseAccessConditions{},
		ModifiedAccessConditions: &azStorageBlob.ModifiedAccessConditions{},
	}
	if leaseID != "" {
		blobAccessConditions.LeaseAccessConditions.LeaseID = &leaseID
	}

	// Sream uploading does not support setting tags because the pages are
	// uploaded in parallel and the tags can only be set once those pages block
	// ids are commited. Use putBlob if you want this behaviour.
	r, err := blockBlobClient.UploadStream(
		ctx,
		reader,
		azStorageBlob.UploadStreamOptions{
			BufferSize:           chunkSize,
			MaxBuffers:           3,
			BlobAccessConditions: &blobAccessConditions,
		},
	)
	if err != nil {
		logger.Sugar.Infof("Cannot upload blob: %v", err)
		return nil, ErrorFromError(err)

	}
	return uploadWriteResponse(r), nil
}

func (azp *Storer) streamReader(
	ctx context.Context,
	identity string,
	r *http.Request,
	options *StorerOptions,
) (*WriteResponse, error) {

	logger.Sugar.Debugf("streamReader: %v", r)
	var err error

	if r.ContentLength < 1 {
		logger.Sugar.Errorf("No content to be uploaded")
		return nil, NewStatusError(fmt.Sprintf("no content to be uploaded"), http.StatusBadRequest)
	}
	// get the multipart reader
	// If the file to be uploaded does not exist, this fails with
	// "request Content-Type isn't multipart/form-data"
	reader, err := r.MultipartReader()
	if err != nil {
		logger.Sugar.Errorf("failed to get multipart reader: %v", err)
		return nil, NewStatusError(fmt.Sprintf("failed to get multipart reader: %v", err), http.StatusBadRequest)
	}

	var resp *WriteResponse

	// we don't know how many files to expect but we only accept one - for now that is
	numFiles := 1
	for {

		part, err := reader.NextPart()
		if err == io.EOF { //nolint https://github.com/golang/go/issues/39155
			// we've got all of it just exit
			logger.Sugar.Debugf("got complete file")
			break
		}

		if err != nil {
			// actual error just log and return
			return nil, NewStatusError(fmt.Sprintf("failed to get next part: %v", err), http.StatusInternalServerError)
		}

		defer part.Close()
		logger.Sugar.Debugf("uploading %s", part.FileName())

		if numFiles > 1 {
			// we got multiple files - bad request
			logger.Sugar.Infof("only one file expected")
			return nil, NewStatusError("only one file expected", http.StatusBadRequest)
		}

		// skip if we don't know what file it is - or if it is file at all
		if part.FileName() == "" {
			continue
		}

		// set up our hashing reader
		hasher := sha256.New()
		uploadData := &hashingReader{
			hasher: hasher,
			part:   part,
		}

		// check we are within the correct size if size limited
		// first check if we have a size limit. -1 is unlimited.
		if options.sizeLimit >= 0 {
			var buffer bytes.Buffer
			counter := io.TeeReader(part, &buffer)

			count, copyErr := io.Copy(io.Discard, counter)
			if copyErr != nil {
				return nil, copyErr
			}

			logger.Sugar.Infof("request file size: %d", count)

			if count >= options.sizeLimit {
				return nil, NewStatusError("filesize exceeds maximum", http.StatusPaymentRequired)
			}
			uploadData.part = &buffer
		}

		// Use mime type if it was supplied.
		var mimeType string
		contentTypeKey := textproto.CanonicalMIMEHeaderKey(ContentKey)
		if len(part.Header[contentTypeKey]) > 0 {
			mimeType = part.Header[contentTypeKey][0] // There should only be one anyway.
		} else {
			// stores bytes required to detect mimetype
			header := bytes.NewBuffer(nil)
			detector := io.TeeReader(uploadData.part, header)

			// after detection bytes used to detect are in header
			// remaining bytes are still in uploadData.part
			m, readerErr := mimetype.DetectReader(detector)
			if readerErr != nil {
				return nil, readerErr
			}
			mimeType = m.String()

			// catenate the header and remaining data to make it look like a new reader
			uploadData.part = io.MultiReader(header, uploadData.part)
		}
		logger.Sugar.Debugf("Mime type is: %s", mimeType)

		// prepare blob
		resp, err = azp.writeStream(ctx, identity, uploadData, options.leaseID)
		if err != nil {
			return nil, err
		}

		// get hash, size and mime type from reader
		var h [sha256.Size]byte
		uploadData.hasher.Sum(h[:0])
		resp.HashValue = hex.EncodeToString(h[:])
		resp.Size = uploadData.size
		resp.MimeType = mimeType
		resp.TimestampAccepted = time.Now().UTC().Format(time.RFC3339)

		// construct metadata
		meta := map[string]string{
			HashKey: resp.HashValue,
			SizeKey: strconv.FormatInt(resp.Size, 10),
			MimeKey: resp.MimeType,
			TimeKey: resp.TimestampAccepted,
		}
		for k, v := range options.metadata {
			meta[k] = v
		}
		// upload metadata
		err = azp.setMetadata(ctx, identity, meta)
		if err != nil {
			return nil, err
		}

		// upload tags
		err = azp.setTags(ctx, identity, options.tags)
		if err != nil {
			return nil, err
		}

		numFiles++
	}
	return resp, nil
}
