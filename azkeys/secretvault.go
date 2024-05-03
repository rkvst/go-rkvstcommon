package azkeys

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-common/tracing"
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

func getWithDefault(key string) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}
	return "<notset>"
}

func EnvironmentAuthorizer() (autorest.Authorizer, error) {

	logger.Sugar.Debugf("Using env authorizer for keyvault")
	logger.Sugar.Debugf("AZURE_TENANT_ID: %s", getWithDefault("AZURE_TENANT_ID"))
	logger.Sugar.Debugf("AZURE_CLIENT_ID: %s", getWithDefault("AZURE_CLIENT_ID"))
	// We do not use the env auhtorizer in production
	logger.Sugar.Debugf("AZURE_CLIENT_SECRET: %s", getWithDefault("AZURE_CLIENT_SECRET"))

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
	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("ReadSecret: %s %s", k.Name, id)

	kvClient, err := NewKvClient(k.Authorizer)
	if err != nil {
		return nil, err
	}

	span, ctx := tracing.StartSpanFromContext(ctx, "KeyVault GetSecret")
	defer span.Finish()

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

func (k *SecretVault) GetOrgKeyHex(
	ctx context.Context, id string,
) (*string, error) {
	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Infof("looking for a secret: %s", id)
	secret, err := k.ReadSecret(ctx, id)
	if err != nil {
		log.Infof("failed to get secret: %v", err)
		return nil, err
	}

	return secret.Value, err
}

// ListSecrets whose id's match prefix and whose tags include all of the
// provided tags
func (k *SecretVault) ListSecrets(
	ctx context.Context, prefix string, tags map[string]string,
) (map[string]SecretEntry, error) {
	log := logger.Sugar.FromContext(ctx)
	defer log.Close()

	log.Debugf("ListSecrets")

	kvClient, err := NewKvClient(k.Authorizer)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	span, ctx := tracing.StartSpanFromContext(ctx, "KeyVault GetSecrets")
	defer span.Finish()

	// must be <= 25
	maxResults := int32(25)
	secrets, err := kvClient.GetSecretsComplete(
		ctx,
		k.Name,
		&maxResults,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets: %w", err)
	}

	span.Finish()

	results := map[string]SecretEntry{}
	for {

		if !secrets.NotDone() {
			break
		}
		item := secrets.Value()

		addr, err := url.Parse(*item.ID)
		if err != nil {
			if err = secrets.NextWithContext(ctx); err != nil {
				return nil, err
			}
			continue
		}

		// Only return secrets with the requested prefix
		if !strings.HasPrefix(strings.TrimPrefix(addr.Path, "/secrets/"), prefix) {
			log.Debugf("`%s' not a prefix of `%s'", prefix, strings.TrimPrefix(addr.Path, "/secrets/"))

			if err = secrets.NextWithContext(ctx); err != nil {
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
				log.Debugf(
					"skipping key, tag `%s' missing or `%v' != `%s'", k, v, matchV)
				matched = false
				break
			}
		}
		if !matched {
			if err = secrets.NextWithContext(ctx); err != nil {
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

		err = secrets.NextWithContext(ctx)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}
