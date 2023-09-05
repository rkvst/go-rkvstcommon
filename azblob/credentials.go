// Package azblob reads/writes files to Azure
// blob storage in Chunks.
package azblob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/rkvst/go-rkvstcommon/logger"
	"github.com/rkvst/go-rkvstcommon/secrets"
)

const (
	listKeyExpand  = ""
	tryTimeoutSecs = 30
)

// credentials gets credentials from env or file
func credentials(
	accountName string,
	resourceGroup string,
	subscription string,
) (*secrets.Secrets, *SharedKeyCredential, error) {

	logger.Sugar.Infof(
		"Attempt environment auth with accountName/resourceGroup/subscription: %s/%s/%s",
		accountName, resourceGroup, subscription,
	)

	if accountName == "" || resourceGroup == "" || subscription == "" {
		return nil, nil, errors.New("missing authentication variables")
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		logger.Sugar.Infof("failed NewAuthorizerFromEnvironment: %v", err)
		return nil, nil, err
	}
	accountClient := storage.NewAccountsClient(subscription)
	accountClient.Authorizer = authorizer

	// Set up a client context to call Azure with
	ctx, cancel := context.WithTimeout(context.Background(), tryTimeoutSecs*time.Second)

	// Even though ctx will be expired, it is good practice to call its
	// cancelation function in any case. Failure to do so may keep the
	// context and its parent alive longer than necessary.
	defer cancel()

	blobkeys, err := accountClient.ListKeys(ctx, resourceGroup, accountName, listKeyExpand)
	if err != nil {
		logger.Sugar.Infof("failed to list blob keys: %v", err)
		return nil, nil, err
	}

	nkeys := len(*blobkeys.Keys)

	if nkeys < 1 {
		return nil, nil, errors.New("no keys found for storage account")
	}
	secret := &secrets.Secrets{
		Account: accountName,
		URL:     fmt.Sprintf("https://%s.blob.core.windows.net/", accountName),
		Key:     *(((*blobkeys.Keys)[0]).Value),
	}
	cred, err := azStorageBlob.NewSharedKeyCredential(secret.Account, secret.Key)
	logger.Sugar.Infof("Credential accountName: %s", cred.AccountName())

	return secret, cred, err
}
