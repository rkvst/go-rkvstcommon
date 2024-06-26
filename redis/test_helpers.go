package redis

import "github.com/go-redis/redis/v8"

func NewResourceWithMockedRedis(
	name string,
	log Logger,
	opt CountResourceOption,
	opts ...ResourceOption,
) (*CountResource, *mockClient) {
	mClient := &mockClient{}
	r := &CountResource{
		Resource: &Resource{
			ClientContext: ClientContext{
				cfg: &clusterConfig{
					Size:           0,
					namespace:      "something",
					clusterOptions: redis.ClusterOptions{},
					options:        redis.Options{},
					log:            log,
				},
				name: name,
			},
			resourceLimiter: defaultResourceLimiter,
			refreshTTL:      defaultTTL,
			refreshCount:    defaultRefreshCount,
			tenantLimits:    make(map[string]*tenantLimit),
			client:          mClient, // Don't use the real thing.
		},
		resourceCounter: defaultResourceCounter,
	}

	// set the resource counter
	opt(r)

	for _, opt := range opts {
		opt(r.Resource)
	}

	return r, mClient
}
