package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	env "github.com/datatrails/go-datatrails-common/environment"
	"github.com/go-redis/redis/v8"
)

const (
	//nolint:gosec
	RedisClusterPassordEnvFileSuffix = "REDIS_STORE_PASSWORD_FILENAME"
	RedisClusterSizeEnvSuffix        = "REDIS_CLUSTER_SIZE"
	RedisNamespaceEnvSuffix          = "REDIS_KEY_NAMESPACE"
	RedisNodeAddressFmtSuffix        = "REDIS_NODE%d_STORE_ADDRESS"
	// The default implementation does  10 * GOMAXPROCS(0). GOMAXPROCS is
	// problematic in containers. Note that each cluster node gets its own pool
	nodePoolSize = 10

	RedisNodeAddressSuffix = "REDIS_STORE_ADDRESS"
	RedisDBSuffix          = "REDIS_STORE_DB"
	RedisPasswordSuffix    = "AZURE_REDIS_STORE_PASSWORD_FILENAME"
)

type RedisConfig interface {
	GetClusterOptions() (*redis.ClusterOptions, error)
	GetOptions() (*redis.Options, error)
	Namespace() string
	IsCluster() bool
	URL() string
	Log() Logger
}

type Scripter interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...any) *redis.Cmd
	ScriptExists(ctx context.Context, hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
}
type RedisClient interface {
	Do(ctx context.Context, args ...any) *redis.Cmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error
	Close() error
	Scripter
}

type clusterConfig struct {
	log            Logger
	Size           int
	namespace      string
	clusterOptions redis.ClusterOptions
	options        redis.Options
}

// ReadClusterConfigOrFatal assumes conventional service env vars and
// populates a ClusterConfig or Fatals out
func FromEnvOrFatal(log Logger) RedisConfig {
	cfg := clusterConfig{log: log}

	cfg.Size = env.GetIntOrFatal(RedisClusterSizeEnvSuffix)
	cfg.namespace = env.GetOrFatal(RedisNamespaceEnvSuffix)

	if cfg.Size == -1 {
		cfg.options.Addr = env.GetOrFatal(RedisNodeAddressSuffix)
		cfg.options.DB = env.GetIntOrFatal(RedisDBSuffix)
		cfg.options.Password = env.ReadIndirectOrFatal(RedisPasswordSuffix)
		return &cfg
	}

	cfg.clusterOptions.Password = env.ReadIndirectOrFatal(RedisClusterPassordEnvFileSuffix)
	cfg.clusterOptions.PoolSize = nodePoolSize
	cfg.clusterOptions.Addrs = make([]string, 0, cfg.Size)
	cfg.clusterOptions.MaxRedirects = cfg.Size
	for i := range cfg.Size {
		suffix := fmt.Sprintf(RedisNodeAddressFmtSuffix, i)
		cfg.clusterOptions.Addrs = append(
			cfg.clusterOptions.Addrs,
			env.GetOrFatal(suffix),
		)
		log.InfoR("Addrs", cfg.clusterOptions.Addrs)
	}

	return &cfg
}

func (cfg *clusterConfig) Log() Logger {
	return cfg.log
}
func (cfg *clusterConfig) IsCluster() bool {
	return cfg.Size > -1
}

func (cfg *clusterConfig) GetClusterOptions() (*redis.ClusterOptions, error) {

	if cfg.IsCluster() {
		return &cfg.clusterOptions, nil
	}

	return nil, fmt.Errorf("unexpected config type when requesting ClusterOptions")
}

func (cfg *clusterConfig) GetOptions() (*redis.Options, error) {

	if !cfg.IsCluster() {
		return &cfg.options, nil
	}

	return nil, fmt.Errorf("unexpected config type when requesting Options")
}

func (cfg *clusterConfig) Namespace() string {
	return cfg.namespace
}

func (cfg *clusterConfig) URL() string {
	if cfg.IsCluster() {
		if len(cfg.clusterOptions.Addrs) == 0 {
			return ""
		}
		return cfg.clusterOptions.Addrs[0]
	}

	return cfg.options.Addr
}

func NewRedisClient(cfg RedisConfig) (RedisClient, error) {
	log := cfg.Log()

	var err error
	if cfg.IsCluster() {
		var copts *redis.ClusterOptions
		if copts, err = cfg.GetClusterOptions(); err != nil {
			return nil, err
		}
		return redis.NewClusterClient(copts), nil
	}

	var opts *redis.Options
	if opts, err = cfg.GetOptions(); err != nil {
		return nil, err
	}
	opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	log.Infof("connecting to redis: %v", opts)
	c := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status := c.Ping(ctx)
	if status.Err() != nil {
		log.Infof("failed ping: %v (%v, %v)", status.Err(), status.FullName(), status.Args())
	}
	return c, status.Err()
}
