// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"

	"github.com/datatrails/go-datatrails-common/logger"
)

const (
	leaseReleaseTimeoutSecs = 5
)

type LeaseRenewer func(ctx context.Context) error

// AcquireLease gets a lease on a blob
func (azp *Storer) AcquireLease(
	ctx context.Context, objectname string, leaseTimeout int32,
) (string, error) {

	logger.Sugar.Debugf("AcquireLease: %v", objectname)
	lease, _, err := azp.acquireLease(ctx, objectname, leaseTimeout)
	herr := ErrorFromError(err)
	if herr.StatusCode() == http.StatusNotFound {
		_, err = azp.Write(ctx, objectname, bytes.NewReader([]byte{}))
		if err != nil {
			logger.Sugar.Infof("failed to create blob %s: %v", objectname, err)
			return "", err
		}
		lease, _, err = azp.acquireLease(ctx, objectname, leaseTimeout)
	}

	if err != nil {
		logger.Sugar.Infof("failed to acquire lease %s: %v", objectname, err)
		return "", herr
	}
	return *lease.LeaseID, nil
}

func (azp *Storer) AcquireLeaseRenewable(
	ctx context.Context, objectname string, leaseTimeout int32,
) (string, LeaseRenewer, error) {
	logger.Sugar.Debugf("AcquireLeaseRenewable: %v", objectname)

	lease, leaseBlobClient, err := azp.acquireLease(
		ctx, objectname, leaseTimeout)
	if err != nil {
		logger.Sugar.Infof("failed to acquire lease %s: %v", objectname, err)
		return "", nil, ErrorFromError(err)
	}

	leaseID := *lease.LeaseID

	renewer := func(ctx context.Context) error {
		renewed, rerr := leaseBlobClient.RenewLease(ctx, nil)
		if rerr != nil {
			logger.Sugar.Infof("failed to renew lease %s: %v", objectname, err)
			return ErrorFromError(rerr)
		}

		renewedID := *renewed.LeaseID
		if renewedID != leaseID {
			// TODO: I think this ought to be a panic, else the api needs to
			// change to recycle the id's
			logger.Sugar.Infof("renew lease mismatch %s: %v", objectname, err)
			return ErrorFromError(fmt.Errorf(
				"renewed lease id mismatch: `%s' != `%s'",
				renewedID, leaseID))
		}
		return nil
	}

	return leaseID, renewer, err
}

func (azp *Storer) acquireLease(
	ctx context.Context, objectname string, leaseTimeout int32,
) (
	*lease.BlobAcquireResponse, *blob.Client, error,
) {
	logger.Sugar.Debugf("acquireLease: %v", objectname)

	blockBlobClient, err := azp.containerClient.NewBlockBlobClient(objectname)
	if err != nil {
		logger.Sugar.Infof("cannot create block blob client %s: %v", objectname, err)
		return nil, nil, err
	}
	leaseBlobClient, err := blockBlobClient.NewBlobLeaseClient(nil)
	if err != nil {
		logger.Sugar.Infof("cannot create lease Blob %s: %v", objectname, err)
		return nil, nil, err
	}
	lease, err := leaseBlobClient.AcquireLease(
		ctx,
		&azStorageBlob.BlobAcquireLeaseOptions{
			Duration: &leaseTimeout,
		},
	)

	return &lease, leaseBlobClient, err
}

// ReleaseLeaseDeferable this is intended to use with defer - doesn't return error so we don't need to check it
func (azp *Storer) ReleaseLeaseDeferable(ctx context.Context, objectname string, leaseID string) {
	logger.Sugar.Debugf("ReleaseLeaseDeferable: %v", objectname)
	err := azp.ReleaseLease(ctx, objectname, leaseID)
	if err != nil {
		logger.Sugar.Infof("did not release lease %s: %v", objectname, err)
	}
}

// ReleaseLease release a lease on a blob
func (azp *Storer) ReleaseLease(ctx context.Context, objectname string, leaseID string,
) error {
	logger.Sugar.Debugf("ReleaseLease: %v", objectname)
	blockBlobClient, err := azp.containerClient.NewBlockBlobClient(objectname)
	if err != nil {
		logger.Sugar.Infof("cannot create block Blob client %s: %v", objectname, err)
		return err
	}
	leaseBlobClient, err := blockBlobClient.NewBlobLeaseClient(&leaseID)
	if err != nil {
		logger.Sugar.Infof("cannot create lease Blob %s: %v", objectname, err)
		return err
	}
	// Releasing a lock is a rare exception to the normal rule for always using the original
	// request context.
	// We want to release the lock even if the request context has timed out or been canceled.
	newCtx, cancel := context.WithTimeout(context.Background(), leaseReleaseTimeoutSecs*time.Second)
	defer cancel()
	_, err = leaseBlobClient.ReleaseLease(newCtx, nil)
	if err != nil {
		logger.Sugar.Infof("failed to release lease %s: %v", objectname, err)
		return ErrorFromError(err)
	}
	return nil
}
