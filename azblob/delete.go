package azblob

import (
	"context"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	msazblob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/datatrails/go-datatrails-common/logger"
)

// Delete the identified blob
func (azp *Storer) Delete(
	ctx context.Context,
	identity string,
) error {
	logger.Sugar.Debugf("Delete blob %s", identity)

	blockBlobClient := azp.containerClient.NewBlockBlobClient(identity)

	_, err := blockBlobClient.Delete(ctx, nil)
	var terr *azcore.ResponseError
	if errors.As(err, &terr) {
		resp := terr.RawResponse
		if resp.Body != nil {
			defer resp.Body.Close()
		}
		if resp.StatusCode == 404 {
			return nil
		}
	}

	return err
}
