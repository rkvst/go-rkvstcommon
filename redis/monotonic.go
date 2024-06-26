package redis

// A monotonically incressing counter which is periodically refreshed from a
// source known to also be monotonic. Intended for tracking organization wallet
// nonces where the reset calls the ledger for the latest nonce. We use a
// refresh on expiry pattern using 'set if not exist' to make this reliable.
//
// This primitive is intended to support ethereum nonce management. To support
// that use case we  implement the following:
//
// 1. The counter is not explicitly initialised
// 2. The first increment will fail because the value does not exist
// 3. We use SETNX to 'set if not exists' so only the first attempt to initialise the counter will succede
// 4. When the value expires, we are back at 1. again
// 5. The setter that is successful returns the value set to the caller as the 'next nonce'
// 6. All other setters call incr again. If this incr indicates the value was
// missing (again) an error is returned
//
// 7. For the specific case of nonce management, The last piece of the puzle is
// once the receipt is successfully obtained a conditional set is issued: set
// current = completed.nonce IF comleted > current
//
// So that we can detect 'not initialised' from INCR we implement the standard
// INCR as a lua script but error in the case where the value doesn't exist. We
// call this INCREX. The implementation is closely derived from go-redis docs
//
// re 6. all other setters call incr again for nonce management: the client will
// make one attempt to refresh. For many racing clients, Exactly one will
// succeede and return the set value as the current nonce.  The remainder will
// each do one further call to INCREX and return the result. Each of those
// clients gets a distinct and sequential nonce. If that second INCREX fails it
// suggests the item expired very quickly or was deleted. In either case we
// return error to the caller. The caller can then retry or not at their
// discression.
//
import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	otrace "github.com/opentracing/opentracing-go"
)

const (
	incrNExNotFound = "not-found"

	// This TTL is our guard against sporadic nonce gaps. These will occur for
	// example if the submition of the transaction to the ledger fails for
	// mundane reasons *after* the nonce is incremented. We have two tacticts
	// for managing nonce gaps to avoid arbitrary delays:
	// 1. The tx issuer spots the error and issues an imediate DEL causing all
	// clients to attepmt to fill the gap
	// 2. The TTL declared here ensures in the face of a crashed client the gap
	// still gets filled withoutt having to wait for the next archivist event on
	// the org wallet.
	//
	// Its not possible to both parallelise the transaction submission *and*
	// guarantee no nonce duplicates or gaps. The duplicates need not be a
	// problem if managed properly - they simply get dropped by the nodes. The
	// nonce-to-low error is completley managable provided transaction
	// preparation is de-coupled from transaction signing. On nonce-to-low,
	// simply get a new nonce and try again.
	monotonicTTL = time.Second * 30
)

// INCREX lua script, derived from  the documented example of regular INCR
// here: https://redis.uptrace.dev/guide/lua-scripting.html#redis-script
//
// In order to detect when the counter value is absent (or expired) we
// define a variant of INCR that errors when the value is absent. See the
// file level comment for how this fits into the broader picture of nonce
// tracking. go-redis automatcailly uses EVALHASH & EVAL to ensure efficient
// management of the script.

// Note: this one is careful to avoid resetting the ttl on set
var incrNEx = redis.NewScript(`
local key = KEYS[1]
local change = tonumber(ARGV[1])

if change < 0 then
  return {err = "increment only"}
end

local value = redis.call("GET", key)
if not value then
  return {err = "not-found"}
end

value = tonumber(value) + change
redis.call("SET", key, value, "KEEPTTL")

return value
`)

// Note: this one is careful to avoid resetting the ttl on set
var setGT = redis.NewScript(`
local key = KEYS[1]
local change = tonumber(ARGV[1])

local value = redis.call("GET", key)

if not value then
  redis.call("SET", key, change)
  return change
end

value = tonumber(value)

if value >= change
  return value
end

redis.call("SET", key, change, "KEEPTTL")

return value
`)

type CountRefresh = func(context.Context, string) (int64, error)

type ScriptRunner interface {
	Run(ctx context.Context, c redis.Scripter, keys []string, args ...any) *redis.Cmd
}

type ScripterClient interface {
	Scripter
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type MonotonicOption func(*Monotonic)

type Monotonic struct {
	ClientContext
	client        ScripterClient
	resetPeriod   time.Duration
	refresh       CountRefresh
	incrNExRunner ScriptRunner // assumed to be incrEx (above)
	setGTRunner   ScriptRunner // assumed to be setGT (above)
}

func NewMonotonic(
	cfg RedisConfig,
	name string,
	opts ...MonotonicOption,
) Monotonic {

	log := cfg.Log()

	log.Debugf("'%s' Resource: '%s'", name, cfg.URL()) // assume at least one addr

	client, err := NewRedisClient(cfg)
	if err != nil {
		log.Panicf("bad redis config provided %v", err)
	}

	c := Monotonic{
		ClientContext: ClientContext{
			cfg:  cfg,
			name: name,
		},
		client:        client,
		resetPeriod:   monotonicTTL,
		incrNExRunner: incrNEx,
		setGTRunner:   setGT,
	}

	for _, opt := range opts {
		opt(&c)
	}

	return c
}

func (c *Monotonic) SetRefresher(refresh CountRefresh) CountRefresh {
	prev := c.refresh
	c.refresh = refresh
	return prev
}

func (c *Monotonic) Name() string {
	return c.name
}

func (c *Monotonic) Log() Logger {
	return c.cfg.Log()
}
func (c *Monotonic) URL() string {
	return c.cfg.URL()
}

func (c *Monotonic) IncrN(ctx context.Context, tenantIDOrWallet string, n int64) (int64, error) {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("IncrN %s %d", tenantIDOrWallet, n)
	n, err := c.incrNEx(ctx, tenantIDOrWallet, n)
	log.Debugf("IncrN = %d: err?=%v", n, err)
	return n, err
}
func (c *Monotonic) SetGT(ctx context.Context, tenantIDOrWallet string, cas int64) (int64, error) {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	log.Debugf("SetGT %s %d", tenantIDOrWallet, cas)
	n, err := c.setGT(ctx, tenantIDOrWallet, cas)
	log.Debugf("SetGT: err?=%v", err)
	return n, err
}

func (c *Monotonic) countPath(tenantIDOrWallet string) string {
	return "Redis monotonic: " + c.URL() + c.countKey(tenantIDOrWallet)
}

func (c *Monotonic) countKey(tenantIDOrWallet string) string {
	return c.cfg.Namespace() + "/counters/" + c.name + "/" + tenantIDOrWallet + "/" + "count"
}

func (c *Monotonic) Del(ctx context.Context, tenantIDOrWallet string) error {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	key := c.countKey(tenantIDOrWallet)

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.Del")
	defer span.Finish()
	log.Debugf("Del %s", tenantIDOrWallet)
	_, err := c.client.Del(ctx, key).Result()
	if err != nil {
		log.Debugf("Redis monotonic: Del %s: %v", key, err)
		return err
	}
	log.Debugf("Del %s ok", tenantIDOrWallet)
	return nil
}

// When used for tracking account nonces, this allows delayed transactions to
// fill gaps and re-sync the nonce cache
func (c *Monotonic) setGT(ctx context.Context, tenantIDOrWallet string, cas int64) (int64, error) {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	key := c.countKey(tenantIDOrWallet)
	path := c.countPath(tenantIDOrWallet)

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.setGT(script)")
	defer span.Finish()

	// count is guaranteed to be the higher of cas or the current value. 'cas' means compare and set.
	count, err := c.setGTRunner.Run(
		ctx, c.client, []string{key}, cas).Int64()
	if err != nil {
		log.Debugf("Redis monotonic: setGT %s: %v", path, err)
		return 0, err
	}
	// Happy path
	return count, nil
}

// setNX runs a `SETNX` operation for redis.
func (c *Monotonic) setNX(ctx context.Context, tenantIDOrWallet string, arg int64) (bool, error) {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	value := parseArg(arg)
	// only pass string arguments to redis

	// find the correct key and path
	key := c.countKey(tenantIDOrWallet)
	path := c.countPath(tenantIDOrWallet)

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.SetNX")
	defer span.Finish()

	log.Debugf("SetNX %s %v", tenantIDOrWallet, arg)
	result, err := c.client.SetNX(ctx, key, value, c.resetPeriod).Result()
	if err != nil {
		log.Debugf("Redis monotonic: NOT SET %s: %s: %v", path, value, err)
		return false, err
	}

	log.Debugf("Redis monotonic: SET %s: %s", path, value)
	return result, nil
}

func (c *Monotonic) incrNEx(ctx context.Context, tenantIDOrWallet string, n int64) (int64, error) {
	log := c.Log().FromContext(ctx)
	defer log.Close()

	key := c.countKey(tenantIDOrWallet)
	path := c.countPath(tenantIDOrWallet)

	span, ctx := otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.INCREX(script)")
	defer span.Finish()

	count, err := c.incrNExRunner.Run(ctx, c.client, []string{key}, n).Int64()
	if err == nil {
		// Happy path
		return count, nil
	}

	// Deal with count refresh/sync
	if err.Error() != incrNExNotFound {
		// actual error rather than the 'not-found' signal
		err = fmt.Errorf("redis monotonic: INCRNEX failed %s: %w", path, err)
		log.Debugf("%v", err)

		return 0, err
	}

	// Value is missing or has never been set. ask for the latest value
	spanRefresh, ctx := otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.INCRNEX(script) count refresh")
	defer spanRefresh.Finish()
	// This means the count did not exist or expired, trigger a refresh
	log.Debugf("Redis monotonic: expired or not initialised - refreshing %s: %v", tenantIDOrWallet, err)
	count, err = c.refresh(ctx, tenantIDOrWallet)
	if err != nil {
		log.Debugf("Redis monotonic: refresh %s: %v", path, err)
		return 0, err
	}

	// issue SETNX. Only one attempt here will succeed assuming:
	// 1. nobody deletes the key manually
	// 2. the expiry isn't rediculously small (as long as its fairly big it doesnt matter if occasionally this happens)
	//
	// In the case where a nonce gap is being force filled by a client, DEL is
	// issued manually, so there is a razor thin chance (meaning it _will_
	// happen from time to time) to instances will get nonce duplicates. Only
	// one of those transactions sharing that duplicate nonce will mine

	// Now we have the current value we add our n, as we need the effect of the
	// operation to be consisten regardless of whether we refreshed. (The
	// current use of this is for claiming 1 or more nonces).
	count += n

	ok, err := c.setNX(ctx, tenantIDOrWallet, count)
	if err != nil {
		// This is a straight up error, the caller sees this as a failed attempt to aquire the count current value
		log.Debugf("Redis monotonic: refresh setNX (ignoring error) %s: %v", path, err)
		return 0, err
	}
	if ok {
		// We are the only client to set the value, it is the correct current
		// value to return to the caller. And we successfully added the 'n'
		log.Debugf("Redis monotonic: refresh setNX ok *(re)initialised* %s: count=%d, added=%d, was=%d", path, count, n, count-n)
		return count, nil
	}

	// Ok, here we are one of the refresh race losers and the count already
	// exists. We issue a single further incr . Here we are claiming our n. All
	// callers, regardless of whether they re-initialised the counter, need
	// their increment to be applied. If we get another not found (or any other
	// error) the client should see this as a transient failure to read the
	// count

	span, ctx = otrace.StartSpanFromContext(ctx, "redis.counter.setOperation.INCREX(script)(2)")
	defer span.Finish()
	count, err = c.incrNExRunner.Run(ctx, c.client, []string{key}, n).Int64()
	if err == nil {
		log.Debugf("Redis monotonic: refresh refresh loser ok %s: count=%d, added=%d, was=%d", path, count, n, count-n)
		return count, nil
	}
	log.Debugf("Redis monotonic: refresh refresh loser error %s: %v", path, err)
	return 0, err
}
