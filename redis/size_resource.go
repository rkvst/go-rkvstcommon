package redis

// Handles creating a size limit.
//
// The underlying size limit is stored in REDIS. The size limit can be retrieved using 'Available()',
//   which first attempts to get the limit via REDIS, then upstream if REDIS is unavailable.

import (
	"context"
	"fmt"

	"github.com/datatrails/go-datatrails-common/logger"
)

type SizeResource struct {
	*Resource
}

// Available gets current size limit.
func (sr *SizeResource) Available(ctx context.Context, tenantID string) (int64, error) {
	return sr.getOperation(ctx, opGetLimit, tenantID)
}

// If limit is less than zero then the limit is disabled and all methods are noops.
func NewSizeResource(
	cfg RedisConfig,
	name string,
	opts ...ResourceOption,
) (*SizeResource, error) {

	logger.Sugar.Debugf("%s SizeResource", name)

	client, err := NewRedisClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client for new size resource: %w", err)
	}

	r := &SizeResource{
		Resource: &Resource{
			ClientContext: ClientContext{
				cfg:  cfg,
				name: name,
			},
			resourceLimiter: defaultResourceLimiter,
			refreshTTL:      defaultTTL,
			refreshCount:    defaultRefreshCount,
			tenantLimits:    make(map[string]*tenantLimit),
			client:          client,
		},
	}

	for _, opt := range opts {
		opt(r.Resource)
	}

	return r, nil
}
