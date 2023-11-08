package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	otrace "github.com/opentracing/opentracing-go"
	"github.com/datatrails/go-datatrails-common/logger"
)

// HashCache uses redis optimistic locking and hash store to cache strings
// safely and prevents race conditions between Get/Set and Delete
// https://redis.io/topics/transactions#optimistic-locking-using-check-and-set
type HashCache struct {
	Cfg RedisConfig
	// the client is long lived and has its own internal pool. There is no strict need to "close"
	client               RedisClient
	expiryTimeoutSeconds int64
	cacheMisses          int64
	cacheHits            int64
}

func NewHashCache(cfg RedisConfig, expiryTimeoutSeconds int64) (*HashCache, error) {
	logger.Sugar.Debugf("Redis NewHashStore")

	client, err := NewRedisClient(cfg)
	if err != nil {
		return nil, err
	}

	c := &HashCache{
		Cfg:    cfg,
		client: client,
	}
	c.expiryTimeoutSeconds = expiryTimeoutSeconds
	return c, nil
}

func (c *HashCache) Close() error {
	logger.Sugar.Debugf("HashCache Close")
	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}

func (c *HashCache) Delete(ctx context.Context, name string) error {
	key := fmt.Sprintf("%s:%s", c.Cfg.Namespace(), name)
	logger.Sugar.Debugf("Delete: %s", key)
	span, ctx := otrace.StartSpanFromContext(ctx, "redis.hashcache.Del")
	defer span.Finish()

	// DEL deletes the hash (HDEL would delete a field)
	_, err := c.client.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	return nil
}

type CachedReader func(results any) error

// CachedRead returns the results from the cache if available. Otherwise the
// reader is invoked and its results are returned instead. The results from the
// reader, if invoked, are cached. This process is transactional. if the (name,
// field) is modified on another connection during the span from read to set, the
// set is not applied.
//
// A note on err handling:
//   - If we fail to read the value from the cache (for any reason) the reader
//     callback is invoked once (and only once) regardless of any other error.
//   - if the reader returns an err that is *always* returned to the caller. this
//     is important as otherwise there is no way to distinguish between a failed
//     read and an empty response from the cache or the db
//   - errors writing to or fetching from the cache are treated as transient and
//     are logged but not returned (persistent cache read/write errors will
//     surface as performance issues or log spam)
//   - errors marshalling or unmarshaling values for the cache are returned to the
//     caller provided a reader error has not occurred.
func (c *HashCache) CachedRead(ctx context.Context, results any, name, field string, reader CachedReader) error {

	key := fmt.Sprintf("%s:%s", c.Cfg.Namespace(), name)

	cacheUpdate := func(tx *redis.Tx, update any) error {
		span, spanCtx := otrace.StartSpanFromContext(ctx, "redis.hashcache.CachedRead.cacheUpdate.TxPipelined")
		defer span.Finish()

		// transactionaly update the cache
		b, err := json.Marshal(update)
		if err != nil {
			logger.Sugar.Infof("Unable to marshal cache update: %v", err)
			return err
		}

		// errors from here are considered transient and logged

		// Operation is committed only if the watched keys remain unchanged.
		_, err = tx.TxPipelined(spanCtx, func(pipe redis.Pipeliner) error {
			pipe.HSet(spanCtx, key, field, b)
			pipe.Expire(spanCtx, key, time.Duration(c.expiryTimeoutSeconds)*time.Second)
			return nil
		})
		if err != nil {
			logger.Sugar.Infof("Unable to set result to cache: %v", err)
		}
		logger.Sugar.Debugf("update cache. key: %s, field: %s, value: %v", key, field, hex.EncodeToString(b))
		return nil
	}

	readerCalledOnce := false
	// In practice we see Watch return nil and yet transact not be called. In
	// this situation we need to guarantee reader is invoked
	cacheHit := false
	var readerErr error // always returned to the caller

	// Note: we have switched from redigo to go-redis. go-redis implements
	// connection pooling, is cluster aware, and has builtin support for
	// pipelining and transactions.
	transact := func(tx *redis.Tx) error {
		span, spanCtx := otrace.StartSpanFromContext(ctx, "redis.hashcache.CachedRead.transact.HGet")
		cachedResults, err := tx.HGet(spanCtx, key, field).Result()
		span.Finish()
		if err != nil {
			c.cacheMisses++
			logger.Sugar.Debugf("unable to read from cache. key: %s, field: %s, expiry: %v, err: %v", key, field, c.expiryTimeoutSeconds, err)

			readerErr = reader(results)
			readerCalledOnce = true // regardless of err, we have called it exactly once
			if readerErr != nil {
				logger.Sugar.Infof("reader failed getting results (for miss): %v", err)
				return readerErr
			}
			return cacheUpdate(tx, results)
		}

		fbytes := []byte(cachedResults)
		if err = json.Unmarshal(fbytes, &results); err != nil {
			c.cacheMisses++
			logger.Sugar.Errorf("unable to unmarshall cached result, err: %v", err)
			readerErr = reader(results)
			readerCalledOnce = true // regardless of err, we have called it exactly once
			if readerErr != nil {
				logger.Sugar.Infof("reader failed getting results (for corrupt entry): %v", err)
				return readerErr
			}

			// with the readerErro dealt with, the only err returned by the
			// cacheUpdate is a marshalling err. We are already handling an
			// unmarshal error here so we return that in preference.
			_ = cacheUpdate(tx, results)
			return err
		}
		c.cacheHits++
		cacheHit = true
		logger.Sugar.Debugf("returning results from cache, hit rate %.2f%% (hits %d misses %d)",
			(100.0*float64(c.cacheHits))/float64(c.cacheHits+c.cacheMisses), c.cacheHits, c.cacheMisses)
		return nil
	}

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.hashcache.CachedRead.Watch")
	defer span.Finish()
	err := c.client.Watch(ctx, transact, key)
	if err != nil || !cacheHit {

		// We need to ensure the reader is called exactly once. And if it errors
		// we must return that error. Otherwise, we return the redis cache
		// response err. In practice we have seen Watch return nil without
		// invoking transact at all, so we need to check explicitly for cachHit
		// in the non err case.
		if err != nil {
			logger.Sugar.Infof("watched transaction failed: %v", err)
		}
		if err == nil {
			logger.Sugar.Infof("go-redis watch returned nil error without a successful cache hit")
		}
		if readerCalledOnce {
			if readerErr != nil {
				return readerErr
			}
			return err
		}
		readerErr = reader(results)
	}

	if readerErr != nil {
		logger.Sugar.Infof("reader failed getting results (for failed watch): %v", readerErr)
	}

	return readerErr
}
