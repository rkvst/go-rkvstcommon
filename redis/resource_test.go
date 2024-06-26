package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	ErrMockResource = errors.New("mockerr")
)

// mockResourceLimit is used to return a mocked resourceLimiter
type mockResourceLimit struct {
	limits         []int64
	errs           []error
	nextLimitIndex int
}

// setup mock limiter
func (mrl *mockResourceLimit) mockResourceLimiter(ctx context.Context, r *Resource, tenantID string) (int64, error) {

	limit := mrl.limits[mrl.nextLimitIndex]
	err := mrl.errs[mrl.nextLimitIndex]

	// increment the limit to return if we can
	if len(mrl.limits) < mrl.nextLimitIndex {
		mrl.nextLimitIndex += 1
	}

	return limit, err
}

// TestLimited tests the Limited method.
//
// We expect the Limited method to return the upstream limit where possible,
//
//	if we do not need to pull from upstream, then we expect Limited to return
//	the redis limit.
//
// If an error occurs getting the upstream limit we expect the redis limit to be returned.
//
// If we can't get either the redis or upstream limit, expect unlimited to be returned.
func TestLimited(t *testing.T) {
	logger.New("NOOP")
	defer logger.OnExit()

	tables := []struct {
		subtest string

		// refresh stuff
		lastRefreshTime time.Time
		refreshTTL      time.Duration // in seconds

		lastRefreshCount int64
		refreshCount     int64

		// mock redis
		redisGetLimit string
		redisGetErr   error
		redisSetErr   error

		// upstream
		upstreamLimit int64
		upstreamErr   error

		// expected
		expected bool
	}{
		{
			"Successful case getting limit from redis",

			time.Now(),
			10000, // make extremely long so we don't trigger upstream

			1,
			10,

			// redis limit
			"10",
			nil,
			nil,

			// upstream shouldn't be hit in this test
			//   but error out if it is.
			-1,
			ErrMockResource,

			// expected
			true,
		},
		{
			"Successful case getting unlimited from redis",

			time.Now(),
			10000, // make extremely long so we don't trigger upstream

			1,
			10,

			// redis limit
			"[-1]",
			nil,
			nil,

			// upstream shouldn't be hit in this test
			//   but error out if it is.
			-1,
			ErrMockResource,

			// expected
			false,
		},
		{
			"Successful case getting limited from upstream, refreshing TTL",

			time.Now(),
			0,

			1,
			10,

			// redis limit
			"[-1]", // make the redis unlimited
			nil,
			nil,

			// upstream should be hit so make limited
			10,
			nil,

			// expected
			true,
		},
		{
			"Successful case getting limited from upstream, refreshing counter",

			time.Now(),
			100000,

			9, // counter should hit 10
			10,

			// redis limit
			"[-1]", // make the redis unlimited
			nil,
			nil,

			// upstream should be hit so make limited
			10,
			nil,

			// expected
			true,
		},
		{
			"Successful case getting limited from upstream, failed to get from redis",

			time.Now(),
			100000,

			1,
			10,

			// redis limit
			"[-1]", // make the redis unlimited
			ErrMockResource,
			nil,

			// upstream should be hit so make limited
			10,
			nil,

			// expected
			true,
		},
		{
			"Successful case getting unlimited from upstream, refresh counter",

			time.Now(),
			100000,

			9, // counter should hit 10
			10,

			// redis limit
			"6", // make the redis limited
			nil,
			nil,

			// upstream should be hit so make unlimited
			-1,
			nil,

			// expected
			false,
		},
		{
			"error from setting new limit should revert to old limit",

			time.Now(),
			100000,

			9, // counter should hit 10
			10,

			// redis limit
			"6", // make the redis limited
			nil,
			ErrMockResource,

			// upstream should be hit so make unlimited
			-1,
			nil,

			// expected
			true,
		},
		{
			"error getting limit from redis and upstream",

			time.Now(),
			100000,

			1,
			10,

			// redis limit
			"0", // make the redis limited
			ErrMockResource,
			nil,

			// upstream should be hit so make limited too
			0,
			ErrMockResource,

			// expected
			false,
		},
		{
			"upstream limit is same as redis limit",

			time.Now(),
			100000,

			9, // counter should hit 10
			10,

			// redis limit
			"801", // make the redis limited
			nil,
			nil,

			// upstream should be hit so make limited too
			801,
			nil,

			// expected
			true,
		},
	}

	for _, table := range tables {
		t.Run(table.subtest, func(t *testing.T) {

			// setup mocking
			mClient := new(mockClient)
			mClient.On("Do", "GET", mock.Anything).Return(redis.NewCmdResult(table.redisGetLimit, table.redisGetErr))
			mClient.On("Set", mock.Anything, mock.Anything, mock.Anything).Return(redis.NewStatusResult("", table.redisSetErr))

			// mock the resource limiter
			mResourceLimit := mockResourceLimit{
				limits: []int64{table.upstreamLimit},
				errs:   []error{table.upstreamErr},
			}
			limits := make(map[string]*tenantLimit)
			limits["tenantID"] = &tenantLimit{
				lastRefreshCount: table.lastRefreshCount,
				lastRefreshTime:  table.lastRefreshTime,
			}

			// create a Resource that we can use
			resource := Resource{
				ClientContext: ClientContext{
					cfg: &clusterConfig{
						namespace: "xxxx",
						log:       logger.Sugar,
					},
				},
				resourceLimiter: mResourceLimit.mockResourceLimiter,
				refreshCount:    table.refreshCount,
				refreshTTL:      table.refreshTTL * time.Second,
				tenantLimits:    limits,

				// add mock client here.
				client: mClient,
			}

			// the ctx doesn't matter as it is mocked away
			actual := resource.Limited(context.TODO(), "tenantID")

			assert.Equal(t, table.expected, actual)
		})
	}
}

// TestLimitedMultipleCalls tests the Limited method after being called multiple times
func TestLimitedMultipleCalls(t *testing.T) {
	logger.New("NOOP")
	defer logger.OnExit()

	type mockRedis struct {
		redisGetLimit    string
		redisGetErr      error
		expectRedisSet   bool
		expectedRedisSet string
		redisSetErr      error
	}
	tables := []struct {
		name string

		// refresh stuff
		lastRefreshTime time.Time
		refreshTTL      time.Duration // in seconds

		lastRefreshCount int64
		refreshCount     int64

		// redis
		mockRedis []mockRedis

		// upstream
		upstreamLimits []int64
		upstreamErrs   []error
	}{
		{
			name: "Successful case getting limit from redis",

			lastRefreshTime: time.Now(),
			refreshTTL:      10000, // make extremely long so we don't trigger upstream

			lastRefreshCount: 0, // start on 0
			refreshCount:     2, // refresh every 2 calls

			// redis limit
			mockRedis: []mockRedis{
				{
					redisGetLimit: "1", // start by saying the limit is 1
					redisGetErr:   nil,

					expectRedisSet: false, // using redis value, no new value set
				},
				{
					redisGetLimit: "1", // limit is still 1
					redisGetErr:   nil,

					expectRedisSet:   true, // we have ticked over the refresh so expect upstream
					expectedRedisSet: "2",
					redisSetErr:      nil,
				},
				{
					redisGetLimit: "2", // limit is now 2
					redisGetErr:   nil,

					expectRedisSet: false, // using redis value, no new value set
				},
				{
					redisGetLimit: "2", // limit is still 2
					redisGetErr:   nil,

					expectRedisSet:   true, // we have ticked over the refresh so expect upstream
					expectedRedisSet: "3",
					redisSetErr:      nil,
				},
				{
					redisGetLimit: "3", // limit is now 3
					redisGetErr:   nil,

					expectRedisSet: false, // using redis value, no new value set
				},
			},

			// upstream shouldn't be hit in this test
			//   but error out if it is.
			upstreamLimits: []int64{2, 3},
			upstreamErrs:   []error{nil, nil},
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {

			// setup mocking
			mClient := new(mockClient)

			for _, mockRedis := range table.mockRedis {
				mClient.On("Do", "GET", mock.Anything).Return(redis.NewCmdResult(mockRedis.redisGetLimit, mockRedis.redisGetErr)).Once()

				if mockRedis.expectRedisSet {
					// expect to set the correct value here
					mClient.On("Set", mock.Anything, mockRedis.expectedRedisSet, mock.Anything).Return(redis.NewStatusResult("", mockRedis.redisSetErr)).Once()
				}
			}

			// mock the resource limiter
			mResourceLimit := mockResourceLimit{
				limits: table.upstreamLimits,
				errs:   table.upstreamErrs,
			}
			limits := make(map[string]*tenantLimit)
			limits["tenantID"] = &tenantLimit{
				lastRefreshCount: table.lastRefreshCount,
				lastRefreshTime:  table.lastRefreshTime,
			}

			// create a Resource that we can use
			resource := Resource{
				ClientContext: ClientContext{
					cfg: &clusterConfig{
						namespace: "xxxx",
						log:       logger.Sugar,
					},
				},
				resourceLimiter: mResourceLimit.mockResourceLimiter,
				refreshCount:    table.refreshCount,
				refreshTTL:      table.refreshTTL * time.Second,
				tenantLimits:    limits,

				// add mock client here.
				client: mClient,
			}

			// check the limit is correctly retrieved
			for range table.mockRedis {

				// the ctx doesn't matter as it is mocked away
				ctx := context.Background()

				// call limited, the assertion is in the mocks above
				resource.Limited(ctx, "tenantID")

			}
		})
	}
}
