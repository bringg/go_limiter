package go_redis_ratelimit

import (
	"errors"
	"time"

	"github.com/go-redis/redis/v7"
)

const (
	DefaultPrefix = "limiter"
	GCRAAlgorithm = iota
	SlidingWindowAlgorithm
	SlidingWindowCloudflareAlgorithm
)

var (
	algorithmNames = map[uint]string{
		GCRAAlgorithm:                    GCRAAlgorithmName,
		SlidingWindowAlgorithm:           SlidingWindowAlgorithmName,
		SlidingWindowCloudflareAlgorithm: SlidingWindowCloudflareAlgorithmName,
	}
	algorithmKeys = map[string]uint{
		GCRAAlgorithmName:                    GCRAAlgorithm,
		SlidingWindowAlgorithmName:           SlidingWindowAlgorithm,
		SlidingWindowCloudflareAlgorithmName: SlidingWindowCloudflareAlgorithm,
	}
)

type (
	Algorithm interface {
		Allow() (*Result, error)
		SetKey(string)
	}

	rediser interface {
		Eval(script string, keys []string, args ...interface{}) *redis.Cmd
		EvalSha(sha1 string, keys []string, args ...interface{}) *redis.Cmd
		ScriptExists(hashes ...string) *redis.BoolSliceCmd
		ScriptLoad(script string) *redis.StringCmd
	}

	Limit struct {
		Algorithm uint
		Rate      int64
		Period    time.Duration
		Burst     int64
	}

	Result struct {
		// Limit is the limit that was used to obtain this result.
		Limit *Limit

		// Key is the key of limit
		Key string

		// Allowed reports whether event may happen at time now.
		Allowed bool

		// Remaining is the maximum number of requests that could be
		// permitted instantaneously for this key given the current
		// state. For example, if a rate limiter allows 10 requests per
		// second and has already received 6 requests for this key this
		// second, Remaining would be 4.
		Remaining int64

		// RetryAfter is the time until the next request will be permitted.
		// It should be -1 unless the rate limit has been exceeded.
		RetryAfter time.Duration

		// ResetAfter is the time until the RateLimiter returns to its
		// initial state for a given key. For example, if a rate limiter
		// manages requests per second and received one request 200ms ago,
		// Reset would return 800ms. You can also think of this as the time
		// until Limit and Remaining will be equal.
		ResetAfter time.Duration
	}
)

// Limiter controls how frequently events are allowed to happen.
type Limiter struct {
	rdb    rediser
	Prefix string
}

// NewLimiter returns a new Limiter.
func NewLimiter(rdb rediser) *Limiter {
	return &Limiter{
		rdb:    rdb,
		Prefix: DefaultPrefix,
	}
}

func (l *Limiter) Allow(key string, limit *Limit) (*Result, error) {
	var algo Algorithm

	switch limit.Algorithm {
	case SlidingWindowAlgorithm:
		algo = &slidingWindow{limit: limit, rdb: l.rdb}
	case SlidingWindowCloudflareAlgorithm:
		algo = &slidingWindoCloudflare{limit: limit, rdb: l.rdb}
	case GCRAAlgorithm:
		algo = &gcra{limit: limit, rdb: l.rdb}
	default:
		return nil, errors.New("algorithm is not supported")
	}

	name, _ := GetAlgorithmName(limit.Algorithm)

	algo.SetKey(l.Prefix + ":" + name + ":" + key)

	return algo.Allow()
}

func GetAlgorithmName(a uint) (string, bool) {
	if name, ok := algorithmNames[a]; ok {
		return name, ok
	}

	return "", false
}

func GetAlgorithmKey(n string) (uint, bool) {
	if key, ok := algorithmKeys[n]; ok {
		return key, ok
	}

	return 0, false
}
