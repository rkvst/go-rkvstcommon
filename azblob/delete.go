package azblob

import (
	"context"
	"errors"

	msazblob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

// Delete the identified blob
func (azp *Storer) Delete(
	ctx context.Context,
	identity string,
) error {
	logger.Sugar.Debugf("Delete blob %s", identity)

	blockBlobClient, err := azp.containerClient.NewBlockBlobClient(identity)
	if err != nil {
		logger.Sugar.Infof("Cannot get block blob client blob: %v", err)
		return ErrorFromError(err)
	}

	_, err = blockBlobClient.Delete(ctx, nil)
	var terr *msazblob.StorageError
	if errors.As(err, &terr) {
		resp := terr.Response()
		if resp.Body != nil {
			defer resp.Body.Close()
		}
		if resp.StatusCode == 404 {
			return nil
		}
	}

	return err
}
