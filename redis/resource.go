package redis

// Handles creating a limit. Use CountResource or SizeResource for specific limits.
//
// The underlying limit is stored in REDIS. The limit can be retrieved using 'getLimit()',
//   which first attempts to get the limit via REDIS, then upstream if REDIS is unavailable.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	otrace "github.com/opentracing/opentracing-go"
)

const (
	// fetch upstream limit if has not been fetched for at least
	// 1 millisecond or for the last 0 creation events.
	// These settings are most inefficient but can be overridden
	// when creating the client.
	defaultTTL          = 1 * time.Millisecond
	defaultRefreshCount = 0
	redisDefaultTTL     = time.Second * 1800

	opSetCount = "SET_Count"
	opGetCount = "GET_Count"
	opIncCount = "INCR_Count"
	opDecCount = "DECR_Count"

	opSetLimit = "SET_Limit"
	opGetLimit = "GET_Limit"
)

var (
	ErrUnknownRedisOperation = errors.New("unknown redis operation")
)

// ResourceLimiter is a function which accepts a resource and retrieves the latest
// caps limit setting from upstream.
type ResourceLimiter = func(context.Context, *Resource, string) (int64, error)

func defaultResourceLimiter(ctx context.Context, r *Resource, tenantID string) (int64, error) {
	return int64(0), nil
}

type tenantLimit struct {
	lastRefreshTime  time.Time
	lastRefreshCount int64
}

// defines a redis client
type Client interface {
	// Do executes a Do command to redis
	Do(ctx context.Context, args ...any) *redis.Cmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Close() error
}

type ClientContext struct {
	cfg  RedisConfig
	name string
}

type Resource struct {
	ClientContext
	client          Client
	resourceLimiter ResourceLimiter
	tenantLimits    map[string]*tenantLimit
	refreshTTL      time.Duration
	refreshCount    int64
}

func (r *Resource) Log() Logger {
	return r.cfg.Log()
}
func (r *Resource) URL() string {
	return r.cfg.URL()
}
func (r *Resource) Name() string {
	return r.name
}

type ResourceOption func(*Resource)

func WithResourceLimiter(resourceLimiter ResourceLimiter) ResourceOption {
	return func(r *Resource) {
		r.resourceLimiter = resourceLimiter
	}
}
func WithRefreshCount(refreshCount int64) ResourceOption {
	return func(r *Resource) {
		r.refreshCount = refreshCount
	}
}
func WithRefreshTTL(refreshTTL time.Duration) ResourceOption {
	return func(r *Resource) {
		r.refreshTTL = refreshTTL
	}
}

func (r *Resource) countPath(tenantID string) string {
	return "Redis Resource " + r.URL() + r.countKey(tenantID)
}

func (r *Resource) countKey(tenantID string) string {
	return r.cfg.Namespace() + "/limits/" + tenantID + "/" + r.name + "/" + "count"
}

func (r *Resource) limitPath(tenantID string) string {
	return "Redis Resource " + r.URL() + r.limitKey(tenantID)
}

func (r *Resource) limitKey(tenantID string) string {
	return r.cfg.Namespace() + "/limits/" + tenantID + "/" + r.name + "/" + "limit"
}

// setOperation runs a specific `SET` operation for redis.
func (r *Resource) setOperation(ctx context.Context, operation string, tenantID string, arg int64) error {
	log := r.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("resource operation %s", operation)

	// only pass string arguments to redis
	strArg := parseArg(arg)

	// find the correct key and path
	var key, path string
	switch operation {
	case opSetCount:
		key = r.countKey(tenantID)
		path = r.countPath(tenantID)
	case opSetLimit:
		key = r.limitKey(tenantID)
		path = r.limitPath(tenantID)
	default:
		return ErrUnknownRedisOperation
	}

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.resource.setOperation.Set")
	defer span.Finish()

	// Don't care about the return value
	_, err := r.client.Set(ctx, key, strArg, redisDefaultTTL).Result()
	if err != nil {
		return err
	}

	log.Debugf("Redis Resource %s: '%s'", operation, path)
	return err
}

// getOperation runs a specific getter operation for redis. This includes
//
//	`INCR`, `DECR` and `GET`.
func (r *Resource) getOperation(ctx context.Context, operation string, tenantID string) (int64, error) {
	log := r.Log().FromContext(ctx)
	defer log.Close()

	// process the operation, expect format to be 'OP_ID' eg. 'GET_limit'
	op := strings.Split(operation, "_")[0]

	// find the correct key and path
	var key, path string
	switch operation {
	case opGetCount, opDecCount, opIncCount:
		key = r.countKey(tenantID)
		path = r.countPath(tenantID)
	case opGetLimit:
		key = r.limitKey(tenantID)
		path = r.limitPath(tenantID)
	default:
		return int64(0), ErrUnknownRedisOperation
	}

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.resource.getOperation.Do")
	defer span.Finish()

	// do the redis operation
	result, err := r.client.Do(ctx, op, key).Int64()
	if err != nil {
		return int64(0), err
	}

	log.Debugf("Redis Resource %s: '%s' %d", operation, path, result)
	return result, nil
}

// parseArg is used to convert int64 to string
//
//	in order to store reliably in redis.
func parseArg(arg int64) string {
	return strconv.FormatInt(arg, 10)
}

// Limited returns true if the limit is enabled. The current limit
// is retrieved from upstream if necessary using the Limiter method.
func (r *Resource) Limited(ctx context.Context, tenantID string) bool {
	log := r.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("Resource Limited %s", tenantID)
	limited := true

	var limits *tenantLimit
	limits, ok := r.tenantLimits[tenantID]
	if !ok {
		log.Debugf("Create tenantLimits %s", tenantID)
		r.tenantLimits[tenantID] = &tenantLimit{
			lastRefreshTime: time.Now(),
		}
		limits = r.tenantLimits[tenantID]
	}
	limits.lastRefreshCount++
	elapsed := time.Since(limits.lastRefreshTime)

	// first attempt to get the current limit for the specific tenant
	limit, err := r.getLimit(ctx, tenantID)
	if err != nil {
		// for now if we can't find the limit we set to unlimited?
		return false
	}

	// we do not need to pull from upstream the local limit will do
	log.Debugf("elapsed %v refreshTTL %v lastRefreshCount %d refreshCount %d",
		elapsed, r.refreshTTL, limits.lastRefreshCount, r.refreshCount,
	)
	if elapsed < r.refreshTTL && limits.lastRefreshCount < r.refreshCount {
		log.Debugf("TTL is still ok %s %d", tenantID, limit)
		return limit >= 0
	}

	// pull from upstream
	log.Infof("Get limit from tenancies service %s %s", r.name, tenantID)
	newLimit, err := r.resourceLimiter(ctx, r, tenantID)
	if err != nil {
		log.Infof("Unable to get upstream limit %s: %v", r.name, err)
		// return whatever the old limit was, as we can't get hold of the new limit
		return limit >= 0
	}

	// reset the refresh triggers
	log.Debugf("Reset the refresh triggers")
	limits.lastRefreshCount = 0
	limits.lastRefreshTime = time.Now()

	// check if the new limit is the same as the old limit
	if newLimit == limit {
		log.Infof("Limit has not changed %s %s", r.name, tenantID)
		return limit >= 0
	}

	log.Infof("new limit now %d (from %d)", newLimit, limit)

	// check if the new limit is unlimited
	if newLimit == -1 {
		limited = false
	}

	// set the new limit
	log.Debugf("Set Redis %s %d", tenantID, limit)
	err = r.setOperation(ctx, opSetLimit, tenantID, newLimit)
	if err != nil {
		log.Infof("failed to set new limit: %v", err)
		// return whatever the old limit was, as we can't set the new limit
		return limit >= 0
	}

	return limited
}

// getLimit gets the limit for a given tenant, attempt first from redis, then upstream.
func (r *Resource) getLimit(ctx context.Context, tenantID string) (int64, error) {
	log := r.Log().FromContext(ctx)
	defer log.Close()

	// if we have found the limit via redis, then return
	log.Debugf("GetLimit From Redis %s", tenantID)
	limit, err := r.getOperation(ctx, opGetLimit, tenantID)
	if err == nil {
		log.Debugf("GetLimit From Redis success %s %d", tenantID, limit)
		return limit, nil
	}
	// we haven't got the redis limit therefore attempt to retrieve from upstream
	return r.RefreshLimit(ctx, tenantID)
}

// RefreshLimit gets the limit for a given tenant from upstream.
func (r *Resource) RefreshLimit(ctx context.Context, tenantID string) (int64, error) {
	log := r.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("RefreshLimit From Tenancies %s", tenantID)
	limit, err := r.resourceLimiter(ctx, r, tenantID)
	if err != nil {
		log.Infof("cannot Get From Tenancies %s %s: %v", tenantID, r.name, err)
		return int64(0), err
	}

	// now we have the upstream value, try and set it in redis
	log.Debugf("SetLimit Redis %s %d", tenantID, limit)
	err = r.setOperation(ctx, opSetLimit, tenantID, limit)
	if err != nil {
		// only log error, as we have the upstream limit
		log.Infof("failed to set limit: %v", err)
	}

	return limit, nil
}

func (r *Resource) nakedWrite(ctx context.Context, count int64, tenantID string) error {
	err := r.setOperation(ctx, opSetCount, tenantID, count)
	if err != nil {
		return DoError(err, r.countPath(tenantID))
	}

	return nil
}

// Close closes the resource and client connection to redis
func (r *Resource) Close() error {
	if r.client != nil {
		return r.client.Close()
	}

	return nil
}
