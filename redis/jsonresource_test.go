package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

// NewTestResource sets up a fresh instance of miniredis and returns a configured JsonResource
func NewTestResource(t *testing.T) JsonResource {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return JsonResource{
		client: c,
		ClientContext: ClientContext{
			cfg:  &clusterConfig{},
			name: "test-json-resource",
		},
		keyPrefix: "resource-prefix",
	}
}

type TestStruct struct {
	Foo string
	Bar int64
}

// TestRoundtrip ensures we can Get a previously Set value, marshalling and unmarshalling as needed.
func TestRoundtrip(t *testing.T) {
	logger.New("NOOP")
	defer logger.OnExit()

	resource := NewTestResource(t)

	setErr := resource.Set(context.TODO(), "tenant/1", &TestStruct{Foo: "hello world", Bar: 1337})
	require.NoError(t, setErr)

	result := &TestStruct{}
	getErr := resource.Get(context.TODO(), "tenant/1", result)
	require.NoError(t, getErr)

	require.Equal(t, "hello world", result.Foo)
	require.Equal(t, int64(1337), result.Bar)
}

// TestExpectedUnmarshalError ensures that Get errors if it cannot unmarshal into the provided
// struct.
func TestExpectedUnmarshalError(t *testing.T) {
	logger.New("NOOP")
	defer logger.OnExit()

	resource := NewTestResource(t)

	setErr := resource.Set(context.TODO(), "tenant/1", &TestStruct{Foo: "hello world", Bar: 1337})
	require.NoError(t, setErr)

	type OtherTestStruct struct {
		Foo int64
		Bar string
	}

	result := &OtherTestStruct{}
	getErr := resource.Get(context.TODO(), "tenant/1", result)
	require.Error(t, getErr)
}
