package azkeys

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
	"github.com/ethereum/go-ethereum/crypto"

	env "github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/logger"
)

const (
	resourceName = "https://vault.azure.net"
)

type SecretVault struct {
	Name       string
	Authorizer autorest.Authorizer // optional, nil for production
}

type SecretEntry struct {
	Identity      string
	VaultIdentity string

	// All the *string entries from the bundle whose values are !nil
	Tags map[string]string

	// Only available via GetSecret
	Value *string
}

// DefaultAuthorizer creates an authorizer which expects to work in cluster
// using an aad-podidentiy to acquire an oauth2 access token. This authorizer is
// used automatically if none is specified on the SecretVault
func DefaultAuthorizer() autorest.Authorizer {
	tokenProvider := &aADProvider{
		resourceName: resourceName,
	}
	return autorest.NewBearerAuthorizer(tokenProvider)
}

func EnvironmentAuthorizer() (autorest.Authorizer, error) {

	logger.Sugar.Infof("Using env authorizer for keyvault")
	logger.Sugar.Infof("AZURE_TENANT_ID: %s", env.GetWithDefault("AZURE_TENANT_ID", "<notset>"))
	logger.Sugar.Infof("AZURE_CLIENT_ID: %s", env.GetWithDefault("AZURE_CLIENT_ID", "<notset>"))
	// We do not use the env auhtorizer in production
	logger.Sugar.Infof("AZURE_CLIENT_SECRET: %s", env.GetWithDefault("AZURE_CLIENT_SECRET", "<notset>"))

	return auth.NewAuthorizerFromEnvironment()
}

// NewKvClient create a keyvault.BaseClient. If auth is nil, the
// DefaultAuthorizer is used
func NewKvClient(authorizer autorest.Authorizer) (keyvault.BaseClient, error) {

	if authorizer == nil {
		var err error
		if authorizer, err = EnvironmentAuthorizer(); err != nil {
			return keyvault.BaseClient{}, err
		}
	}

	kvClient := keyvault.New()
	kvClient.Authorizer = authorizer

	return kvClient, nil
}

// ReadSecret returns the identified secret metadata and value
func (k *SecretVault) ReadSecret(
	ctx context.Context, id string,
) (*SecretEntry, error) {

	logger.Sugar.Infof("ReadSecret: %s %s", k.Name, id)

	kvClient, err := NewKvClient(k.Authorizer)
	if err != nil {
		return nil, err
	}

	secret, err := kvClient.GetSecret(ctx, k.Name, id, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}
	// We can assume ID != nil, if GetSecret returns without error
	entry := &SecretEntry{
		Identity:      id,
		VaultIdentity: *secret.ID,
		Value:         secret.Value,
		Tags:          make(map[string]string),
	}

	for k, v := range secret.Tags {
		if v != nil {
			entry.Tags[k] = *v
		}
	}

	return entry, nil
}

func (k *SecretVault) GetOrgKey(
	ctx context.Context, id string,
) (*ecdsa.PrivateKey, error) {

	logger.Sugar.Infof("looking for a secret: %s", id)
	secret, err := k.ReadSecret(ctx, id)
	if err != nil {
		logger.Sugar.Infof("failed to get secret: %v", err)
		return nil, err
	}

	r, err := crypto.HexToECDSA(strings.TrimPrefix(*secret.Value, "0x"))
	if err != nil {
		logger.Sugar.Infof("could not do ecdsa from key: %v", err)
	}

	return r, err
}

// ListSecrets whose id's match prefix and whose tags include all of the
// provided tags
func (k *SecretVault) ListSecrets(
	ctx context.Context, prefix string, tags map[string]string,
) (map[string]SecretEntry, error) {

	logger.Sugar.Debugf("ListSecrets")

	kvClient, err := NewKvClient(k.Authorizer)
	if err != nil {
		return nil, err
	}

	ctxo, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// must be <= 25
	maxResults := int32(25)
	secrets, err := kvClient.GetSecretsComplete(
		ctxo,
		k.Name,
		&maxResults,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets: %w", err)
	}

	results := map[string]SecretEntry{}
	for {

		if !secrets.NotDone() {
			break
		}
		item := secrets.Value()

		addr, err := url.Parse(*item.ID)
		if err != nil {
			if err = secrets.NextWithContext(ctxo); err != nil {
				return nil, err
			}
			continue
		}

		// Only return secrets with the requested prefix
		if !strings.HasPrefix(strings.TrimPrefix(addr.Path, "/secrets/"), prefix) {
			logger.Sugar.Debugf("`%s' not a prefix of `%s'", prefix, strings.TrimPrefix(addr.Path, "/secrets/"))

			if err = secrets.NextWithContext(ctxo); err != nil {
				return nil, err
			}
			continue
		}

		// Only return secrets whose tags contain ALL of the requested
		// tags.
		matched := true // empty tags is wild
		for k, matchV := range tags {
			v, ok := item.Tags[k]
			if !ok || v == nil || matchV != *v {
				logger.Sugar.Debugf(
					"skipping key, tag `%s' missing or `%v' != `%s'", k, v, matchV)
				matched = false
				break
			}
		}
		if !matched {
			if err = secrets.NextWithContext(ctxo); err != nil {
				return nil, err
			}
			continue
		}

		id := strings.TrimPrefix(addr.Path, "/secrets/")
		results[id] = SecretEntry{
			Identity:      id,
			VaultIdentity: *item.ID,
			Tags:          map[string]string{},
		}
		for k, v := range item.Tags {
			if v != nil {
				results[id].Tags[k] = *v
			}
		}

		err = secrets.NextWithContext(ctxo)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}
