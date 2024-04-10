package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/datatrails/go-datatrails-common/logger"
	otrace "github.com/opentracing/opentracing-go"
)

// NewJsonResource constructs a new instance of JsonResource, given the configuration.
//   - name: name of the resource
//   - resType: the general type of resource, mostly to help with organisation
//   - cfg: the Redis cluster configuration to use
func NewJsonResource(name string, cfg RedisConfig, resType string) (*JsonResource, error) {
	client, err := NewRedisClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create redis client: %w", err)
	}

	return &JsonResource{
		ClientContext: ClientContext{
			cfg:  cfg,
			name: name,
		},
		client:    client,
		keyPrefix: fmt.Sprintf("%s/%s/%s", cfg.Namespace(), resType, name),
	}, nil
}

// JsonResource is a Redis resource holding an object in JSON
type JsonResource struct {
	ClientContext
	client    Client
	keyPrefix string
}

// URL gets the configured Redis URL
func (r *JsonResource) URL() string {
	return r.cfg.URL()
}

// Name gets the name of the resource
func (r *JsonResource) Name() string {
	return r.name
}

// Key gets the full resource identification key
func (r *JsonResource) Key(tenantID string) string {
	return r.keyPrefix + "/" + tenantID
}

// Set takes a JSON-serializable value, marshals it, and stores it for the given tenantID
func (r *JsonResource) Set(ctx context.Context, tenantID string, value any) error {
	span, ctx := otrace.StartSpanFromContext(ctx, "redis.resource.setOperation.Set")
	defer span.Finish()

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	_, err = r.client.Set(ctx, r.Key(tenantID), string(jsonBytes), redisDefaultTTL).Result()
	if err != nil {
		return err
	}

	logger.Sugar.Debugf("Set: set resource '%s' to '%s'", r.Key(tenantID), value)
	return nil
}

// Get reads the resource for the given tenantID, and unmarshals it into target (which must be a
// pointer to a suitable empty struct.)
func (r *JsonResource) Get(ctx context.Context, tenantID string, target any) error {
	span, ctx := otrace.StartSpanFromContext(ctx, "redis.resource.getOperation.Do")
	defer span.Finish()

	result, err := r.client.Do(ctx, "GET", r.Key(tenantID)).Result()
	if err != nil {
		logger.Sugar.Infof("Get: error getting result for %s: %v", r.Key(tenantID), err)
		return err
	}

	resultStr, ok := result.(string)
	if !ok {
		return fmt.Errorf("could not interpret result for: %s as string", r.Key(tenantID))
	}

	err = json.Unmarshal([]byte(resultStr), target)
	if err != nil {
		return err
	}

	return nil
}
