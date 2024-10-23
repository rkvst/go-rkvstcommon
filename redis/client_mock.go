package redis

// Defines Mocks for redis ClusterClient

import (
	"time"

	"context"

	"github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/mock"
)

// mockClient is a mock redis Client
type mockClient struct {
	mock.Mock
}

func (mc *mockClient) Do(ctx context.Context, args ...any) (reply *redis.Cmd) {

	arguments := mc.Called(args...)
	return arguments.Get(0).(*redis.Cmd)
}

func (mc *mockClient) Set(ctx context.Context, key string, value any, expiration time.Duration) (reply *redis.StatusCmd) {

	arguments := mc.Called(key, value, expiration)
	return arguments.Get(0).(*redis.StatusCmd)
}

func (mc *mockClient) Close() error {
	arguments := mc.Called()
	return arguments.Get(0).(error)
}

func (mc *mockClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (reply *redis.BoolCmd) {

	arguments := mc.Called(key, value, expiration)
	return arguments.Get(0).(*redis.BoolCmd)
}
