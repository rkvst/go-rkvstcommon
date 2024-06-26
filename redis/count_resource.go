package redis

import (
	"context"
	"time"
)

// ResourceCounter is a function which accepts a context and counts the number of
// reources in the database.
type ResourceCounter = func(context.Context) (int64, error)

func defaultResourceCounter(ctx context.Context) (int64, error) {
	return int64(0), nil
}

// CountResource - the counter is initialised with a maximum value and as
// resources are created the counter is decremented. Additionally when the counter reaches
// zero the actual number of resources is counted and the current count adjusted
// accordingly.
type CountResource struct {
	*Resource

	resourceCounter ResourceCounter
	name            string
	limit           int64
}

type CountResourceOption func(*CountResource)

func WithResourceCounter(resourceCounter ResourceCounter) CountResourceOption {
	return func(cr *CountResource) {
		cr.resourceCounter = resourceCounter
	}
}

// NewResource - creates pool of connections to redis that manages a decrementing counter.
// If limit is less than zero then the counter is disabled and all methods are noops.
func NewResource(
	cfg RedisConfig,
	name string,
	opt CountResourceOption, // to set the resource counter
	opts ...ResourceOption,
) *CountResource {

	log := cfg.Log()
	log.Debugf("'%s' Resource: '%s'", name, cfg.URL()) // assume at least one addr

	client, err := NewRedisClient(cfg)
	if err != nil {
		log.Panicf("bad redis config provided %v", err)
	}

	r := &CountResource{
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
		resourceCounter: defaultResourceCounter,
	}

	// set the resource counter
	opt(r)

	for _, opt := range opts {
		opt(r.Resource)
	}

	return r
}

func (cr *CountResource) Log() Logger {
	return cr.Resource.Log()
}
func (cr *CountResource) Name() string {
	return cr.Resource.name
}

// Return adds 1 to counter
func (cr *CountResource) Return(ctx context.Context, tenantID string) error {

	log := cr.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("Return: '%s'", cr.countPath(tenantID))
	limit, err := cr.getLimit(ctx, tenantID)
	if err != nil {
		return err
	}

	// if we are unlimited its fine
	if limit < 0 {
		return nil
	}

	_, err = cr.getOperation(ctx, opIncCount, tenantID)
	if err != nil {
		// If we cannot increment the current counter then try and calculate the
		// actual value.... Inefficient but should happen infrequently.
		_, err = cr.initialise(ctx, tenantID, limit)
		if err != nil {
			return err
		}
	}
	return nil
}

// Consume subtracts 1 from counter
func (cr *CountResource) Consume(ctx context.Context, tenantID string) error {

	log := cr.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("Consume: '%s'", cr.countPath(tenantID))
	limit, err := cr.getLimit(ctx, tenantID)
	if err != nil {
		return err
	}

	if limit <= 0 {
		return nil
	}

	_, err = cr.getOperation(ctx, opDecCount, tenantID)
	if err != nil {
		// If we cannot decrement the current counter then try and calculate the
		// actual value.... Inefficient but should happen infrequently.
		_, err = cr.initialise(ctx, tenantID, limit)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetLimit gets the current limit
func (cr *CountResource) GetLimit() int64 {
	return cr.limit
}

// Limited returns true if the limit counter is enabled. The current limit
// is retrieved from upstream if necessary using the Limiter method. This happens
// when the current TTL parameters are exceeded.
func (cr *CountResource) Limited(ctx context.Context, tenantID string) bool {
	log := cr.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("Limited: '%s'", cr.countPath(tenantID))
	limited := true

	var limits *tenantLimit
	limits, ok := cr.tenantLimits[tenantID]
	if !ok {
		log.Debugf("Create tenantLimits %s", tenantID)
		cr.tenantLimits[tenantID] = &tenantLimit{
			lastRefreshTime: time.Now(),
		}
		limits = cr.tenantLimits[tenantID]
	}
	limits.lastRefreshCount++
	elapsed := time.Since(limits.lastRefreshTime)

	// first attempt to get the current limit for the specific tenant
	limit, err := cr.getLimit(ctx, tenantID)
	if err != nil {
		// for now if we can't find the limit we set to unlimited?
		return false
	}

	// stash the limit
	cr.limit = limit

	// we do not need to pull from upstream the local limit will do
	log.Debugf("elapsed %v refreshTTL %v lastRefreshCount %d refreshCount %d",
		elapsed, cr.refreshTTL, limits.lastRefreshCount, cr.refreshCount,
	)
	if elapsed < cr.refreshTTL && limits.lastRefreshCount < cr.refreshCount {
		log.Debugf("TTL is still ok %s %d", tenantID, limit)
		return limit >= 0
	}

	// pull from upstream
	log.Infof("Get limit from tenancies service %s %s", cr.name, tenantID)
	newLimit, err := cr.resourceLimiter(ctx, cr.Resource, tenantID)
	if err != nil {
		log.Infof("Unable to get upstream limit %s: %v", cr.name, err)
		// return whatever the old limit was, as we can't get hold of the new limit
		return limit >= 0
	}

	// reset the refresh triggers
	log.Debugf("Reset the TTL triggers")
	limits.lastRefreshCount = 0
	limits.lastRefreshTime = time.Now()
	cr.limit = newLimit

	// check if the new limit is the same as the old limit
	if newLimit == limit {
		log.Infof("Limit has not changed %s %s", cr.name, tenantID)
		return limit >= 0
	}

	log.Infof("new limit now %d (from %d)", newLimit, limit)

	// check if the new limit is unlimited
	if newLimit == -1 {
		limited = false
	}

	// if we go from unlimited to limited we need to populate the counter
	// if we decrease the caps limit we need to re-populate the counter
	if (limit == -1 && newLimit >= 0) || (newLimit < limit && newLimit != -1) {
		log.Infof("Populate counter %d (from %d)", newLimit, limit)
		_, err = cr.initialise(ctx, tenantID, newLimit)
		if err != nil {
			log.Infof("reset count failure: %v", err)
		}
	}

	// set the new limit
	log.Debugf("Set Redis %s %d", tenantID, limit)
	err = cr.setOperation(ctx, opSetLimit, tenantID, newLimit)
	if err != nil {
		log.Infof("failed to set new limit: %v", err)
		// return whatever the old limit was, as we can't set the new limit
		return limit >= 0
	}

	return limited

}

// initialise - sets counter with initial value if it does not exist or is
// (temporarily inaccessible).
// Subtracts current number of resources determined by executing the Counter Method.
func (cr *CountResource) initialise(ctx context.Context, tenantID string, limit int64) (int64, error) {
	log := cr.Log().FromContext(ctx)
	defer log.Close()

	var err error

	log.Debugf("Initialise: '%s'", cr.countPath(tenantID))
	currentCount, err := cr.resourceCounter(ctx)
	if err != nil {
		log.Infof("currentCount failure: %v", err)
		return int64(0), err
	}
	log.Debugf("Redis Resource Current Count: '%s' %d", cr.countPath(tenantID), currentCount)
	var count int64
	if currentCount >= limit {
		count = 0
	} else {
		count = limit - currentCount
	}

	err = cr.nakedWrite(ctx, count, tenantID)
	// Log but ignore SET failure as we have enough info to enforce the limit without
	// REDIS.
	if err != nil {
		log.Infof("Redis Resource SET failure: %v", err)
	}
	log.Debugf("Resource Initialised: '%s' %d", cr.countPath(tenantID), count)

	return count, nil
}

// Available gets current value of counter and adjusts limit if required.
// An error is returned when either the redis counter cannot be read or this
// counter is disabled.
func (cr *CountResource) Available(ctx context.Context, tenantID string) (int64, error) {
	log := cr.Log().FromContext(ctx)
	defer log.Close()

	var err error

	log.Debugf("Available: '%s'", cr.countPath(tenantID))
	limit, err := cr.getLimit(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	count, err := cr.getOperation(ctx, opGetCount, tenantID)
	if err != nil {
		// If we cannot get the current counter then try and calculate the
		// actual value.... Inefficient but should happen infrequently.
		count, err = cr.initialise(ctx, tenantID, limit)
		if err != nil {
			return int64(0), err
		}
	}
	log.Infof("Counter: %d", count)

	if count > 0 {
		return count, nil
	}

	// count is now zero so check the actual number of resources and adjust if necessary.
	newLimit, err := cr.refreshLimit(ctx, tenantID)
	if err != nil {
		log.Infof("reset count failure: %v", err)
		newLimit = limit
	}
	count, err = cr.initialise(ctx, tenantID, newLimit)
	if err != nil {
		log.Infof("reset count failure: %v", err)
	}
	return count, nil
}

// ReadOnlyAvailable
//
// Returns the number of available units of this resource. Attempts the fast path first, of getting
// from the cache directly. Will attempt to re-initialise the count if its inaccessible or 0.
func (cr *CountResource) ReadOnlyAvailable(ctx context.Context, tenantID string) (int64, error) {
	log := cr.Log().FromContext(ctx)
	defer log.Close()

	var err error

	log.Debugf("ReadOnlyAvailable: '%s'", cr.countPath(tenantID))
	// Pure read operation from the cache - will usually succeed and return a non-zero value.
	count, err := cr.getOperation(ctx, opGetCount, tenantID)
	if err != nil || count == 0 {
		if err != nil {
			log.Infof("error getting count from Redis: %v", err)
		}

		// An error might indicate that the key wasn't available in the cache, so we use the
		// full-fat method to initialise it. We also want to do this if the count has hit 0, since
		// the user may have paid to increase their cap for this resource.

		log.Infof("ReadOnlyAvailable: Insufficient resource: Delegating to Available")
		return cr.Available(ctx, tenantID)
	}

	log.Debugf("ReadOnlyAvailable: Counter: %d", count)
	return count, nil
}
