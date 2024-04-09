package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/datatrails/go-datatrails-common/logger"
	otrace "github.com/opentracing/opentracing-go"
)

func NewSimpleResource(name string, cfg RedisConfig, resType string) (*SimpleResource, error) {
	client, err := NewRedisClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create redis client: %w", err)
	}

	return &SimpleResource{
		ClientContext: ClientContext{
			cfg:  cfg,
			name: name,
		},
		client:    client,
		keyPrefix: fmt.Sprintf("%s/%s/%s", cfg.Namespace(), resType, name),
	}, nil
}

// SimpleResource
type SimpleResource struct {
	ClientContext
	client    Client
	keyPrefix string
}

func (r *SimpleResource) URL() string {
	return r.cfg.URL()
}

func (r *SimpleResource) Name() string {
	return r.name
}

func (r *SimpleResource) Key(tenantID string) string {
	return r.keyPrefix + "/" + tenantID
}

func (r *SimpleResource) Set(ctx context.Context, tenantID string, value any) error {
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

func (r *SimpleResource) Get(ctx context.Context, tenantID string, target any) error {
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
