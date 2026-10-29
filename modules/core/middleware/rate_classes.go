package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
)

// loginRoutes are the sign-ins: each guess at a password counts against the IP and the address tried.
var loginRoutes = map[string]bool{
	"POST /api/v1/auth/login": true,
}

// heavyRoutes are the requests that read or write thousands of rows at once — imports today, exports and
// reports when they come: a tenant runs a few a minute, so one business's import never slows the rest.
var heavyRoutes = map[string]bool{
	"POST /api/v1/import":          true,
	"POST /api/v1/import/detect":   true,
	"POST /api/v1/import/validate": true,
}

// publicRoutes answer anyone holding a link, with no session (TRACK-060): a page polls them, so each
// IP gets its own minute budget.
var publicRoutes = map[string]bool{
	"GET /api/v1/public/share/:token": true,
}

// deviceRoutes are a hardware tracker's (TRACK-068): many share a carrier's IP, so the per-IP limit
// skips them and each token gets its own minute budget instead.
var deviceRoutes = map[string]bool{
	"GET /api/v1/gps/osmand":  true,
	"POST /api/v1/gps/osmand": true,
}

// RateClasses adds the stricter classes on top of the per-IP limit (INFRA-011 D3): sign-in at
// loginPerMin per IP and e-mail, heavy work at heavyPerMin per tenant, the public reads at publicPerMin
// per IP, a tracker's points at devicePerMin per token. One Valkey call, only on those routes; without
// Valkey, or when it fails, the request passes (INFRA-001 D5) and the general limit still applies.
func RateClasses(valkey coreServices.ValkeyService, loginPerMin, heavyPerMin, publicPerMin, devicePerMin int) gin.HandlerFunc {
	if valkey == nil {
		return func(c *gin.Context) { c.Next() }
	}
	limiter := redis_rate.NewLimiter(valkey.Client())
	login := redis_rate.PerMinute(loginPerMin)
	heavy := redis_rate.PerMinute(heavyPerMin)
	public := redis_rate.PerMinute(publicPerMin)
	device := redis_rate.PerMinute(devicePerMin)

	return func(c *gin.Context) {
		route := c.Request.Method + " " + c.FullPath()
		var key string
		var limit redis_rate.Limit
		switch {
		case loginRoutes[route]:
			key, limit = "login:"+c.ClientIP()+":"+loginEmail(c), login
		case heavyRoutes[route] && c.GetHeader("X-Tenant-Slug") != "":
			key, limit = "heavy:"+c.GetHeader("X-Tenant-Slug"), heavy
		case publicRoutes[route]:
			key, limit = "public:"+c.ClientIP(), public
		case deviceRoutes[route]:
			sum := sha256.Sum256([]byte(c.Query("id")))
			key, limit = "device:"+hex.EncodeToString(sum[:]), device
		default:
			c.Next()
			return
		}
		res, err := limiter.Allow(c.Request.Context(), key, limit)
		if err != nil || res.Allowed > 0 {
			c.Next()
			return
		}
		c.Header("Retry-After", strconv.Itoa(int(res.RetryAfter/time.Second)+1))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, coreErrors.BuildErrorSingle(c, coreErrors.RequestRateLimited))
	}
}

// loginEmail reads the address from a sign-in body and puts the body back for the controller.
func loginEmail(c *gin.Context) string {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<16))
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	var body struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &body)
	return strings.ToLower(strings.TrimSpace(body.Email))
}
