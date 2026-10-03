package middleware

import (
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
)

// rateLimitWarnEvery spaces the fail-open warnings: a Valkey outage must not write one log line per
// request.
const rateLimitWarnEvery = time.Minute

// RateLimit caps requests per client IP at perSecond sustained, with bursts of up to burst at once
// (INFRA-010 D2: a page firing a dozen reads together still loads). With Valkey the window is shared by every
// replica (GCRA, one round-trip per request); without it the limit is the in-memory tollbooth one
// and holds per process only.
//
// A Valkey error lets the request through (INFRA-001 D5): the limit protects the API, and refusing
// every request because the limiter is down would be the outage it exists to prevent.
func RateLimit(valkey coreServices.ValkeyService, perSecond, burst int) gin.HandlerFunc {
	if valkey == nil {
		limiter := tollbooth.NewLimiter(float64(perSecond), nil)
		limiter.SetTokenBucketExpirationTTL(time.Second)
		limiter.SetBurst(burst)
		limiter.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
		return tollbooth_gin.LimitHandler(limiter)
	}

	limiter := redis_rate.NewLimiter(valkey.Client())
	limit := redis_rate.Limit{Rate: perSecond, Burst: burst, Period: time.Second}
	var lastWarn atomic.Int64

	return func(c *gin.Context) {
		// ClientIP honours X-Forwarded-For only from the trusted proxies, so a client cannot pick a
		// fresh key per request by forging the header.
		res, err := limiter.Allow(c.Request.Context(), "ip:"+c.ClientIP(), limit)
		if err != nil {
			now := time.Now().UnixNano()
			if last := lastWarn.Load(); now-last >= int64(rateLimitWarnEvery) && lastWarn.CompareAndSwap(last, now) {
				log.Printf("⚠️  rate limit: Valkey unavailable, failing open: %v", err)
			}
			c.Next()
			return
		}
		if res.Allowed == 0 {
			c.Header("Retry-After", strconv.Itoa(int(res.RetryAfter/time.Second)+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, coreErrors.BuildErrorSingle(c, coreErrors.RequestRateLimited))
			return
		}
		c.Next()
	}
}
