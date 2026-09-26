package realtime

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	tenancyModels "josex/web/modules/tenancy/models"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// bearerProtocol is the subprotocol a browser authenticates with: it cannot set Authorization on a
// WebSocket, so it offers ["bearer", "<jwt>"] and the server accepts "bearer".
const bearerProtocol = "bearer"

// authTTL is how long one subscribe answer is trusted per (user, channel).
const authTTL = 5 * time.Minute

// writeTimeout bounds one frame's write; a client that cannot take it is gone.
const writeTimeout = 10 * time.Second

// SubscribeChecker is sp_can_subscribe, run in the session's tenant database.
type SubscribeChecker func(ctx context.Context, session Session, channel string) (bool, error)

// BrowserAuth lets a browser reach the socket behind the REST chain: the JWT offered as the
// subprotocol pair becomes the Authorization header, ?tenant_slug= the X-Tenant-Slug header — the
// tenant chain still checks the membership, exactly as for the header. A request
// that already carries the headers is left as it is. It runs before the auth middleware.
func BrowserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if token := offeredBearer(c.Request); token != "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
		}
		if slug := c.Query("tenant_slug"); slug != "" && c.GetHeader("X-Tenant-Slug") == "" {
			c.Request.Header.Set("X-Tenant-Slug", slug)
		}
		c.Next()
	}
}

func offeredBearer(r *http.Request) string {
	var protocols []string
	for _, h := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(h, ",") {
			protocols = append(protocols, strings.TrimSpace(p))
		}
	}
	for i, p := range protocols {
		if p == bearerProtocol && i+1 < len(protocols) {
			return protocols[i+1]
		}
	}
	return ""
}

// OriginPatterns turns the CORS origins (scheme://host:port) into the host patterns the upgrade
// checks the browser's Origin against.
func OriginPatterns(allowed []string) []string {
	out := make([]string, 0, len(allowed))
	for _, o := range allowed {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		} else if o != "" {
			out = append(out, o)
		}
	}
	return out
}

type clientOp struct {
	Op      string `json:"op"`
	Channel string `json:"channel"`
}

// Serve is GET /tracking/ws, mounted behind the auth and tenant chain: upgrade, then one reader
// (subscribe/unsubscribe), one writer (the connection's queue) and a ping every pingEvery.
func (h *Hub) Serve(check SubscribeChecker, origins []string, pingEvery time.Duration) gin.HandlerFunc {
	cache := newAuthCache()
	return func(c *gin.Context) {
		session, ok := sessionOf(c)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			Subprotocols:   []string{bearerProtocol},
			OriginPatterns: origins,
		})
		if err != nil {
			return
		}
		conn := NewConn(session, h.queueMax)
		ctx, cancel := context.WithCancel(h.ctx)
		defer cancel()
		defer h.Remove(conn)

		go h.read(ctx, ws, conn, check, cache)
		h.write(ctx, ws, conn, pingEvery)
	}
}

// sessionOf reads who is connecting from the tenant chain. Operators and guardians listen; the
// organization and driver levels have no channel of their own yet.
func sessionOf(c *gin.Context) (Session, bool) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return Session{}, false
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return Session{}, false
	}
	role := c.GetString("tenant_user_role")
	if role == tenancyModels.RoleOrganization || role == tenancyModels.RoleDriver {
		return Session{}, false
	}
	session := Session{TenantID: tenantID, UserID: userID, Guardian: role == tenancyModels.RolePortal}
	if raw, ok := c.Get("tenant_database_url"); ok {
		if dbURL, ok := raw.(*string); ok && dbURL != nil {
			session.DatabaseURL = *dbURL
		}
	}
	return session, true
}

// read handles the client's ops until the socket fails; a refused subscribe answers an error frame
// and keeps the socket.
func (h *Hub) read(ctx context.Context, ws *websocket.Conn, conn *Conn, check SubscribeChecker, cache *authCache) {
	defer conn.Close(websocket.StatusNormalClosure, "")
	for {
		_, raw, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var op clientOp
		if json.Unmarshal(raw, &op) != nil || op.Channel == "" {
			conn.Refuse(op.Channel, "bad-request")
			continue
		}
		switch op.Op {
		case "subscribe":
			allowed, err := cache.allowed(ctx, conn.Session, op.Channel, check)
			if err != nil {
				log.Printf("⚠️  realtime: subscribe check %s: %v", op.Channel, err)
				conn.Refuse(op.Channel, "unavailable")
				continue
			}
			if !allowed {
				conn.Refuse(op.Channel, "forbidden")
				continue
			}
			if err := h.Subscribe(conn, op.Channel); err != nil {
				conn.Refuse(op.Channel, "unavailable")
			}
		case "unsubscribe":
			h.Unsubscribe(conn, op.Channel)
		default:
			conn.Refuse(op.Channel, "bad-request")
		}
	}
}

// write sends what the connection queued, pings on schedule, and closes the socket with the
// connection's code when it is closed — by the hub (1012, 1013), the reader, or a failed ping.
func (h *Hub) write(ctx context.Context, ws *websocket.Conn, conn *Conn, pingEvery time.Duration) {
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-conn.Done():
			code, reason, _ := conn.Closed()
			_ = ws.Close(code, reason)
			return
		case <-ctx.Done():
			_ = ws.Close(StatusServiceRestart, "server restarting")
			return
		case <-ping.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingEvery)
			err := ws.Ping(pingCtx)
			cancel()
			if err != nil {
				conn.Close(websocket.StatusPolicyViolation, "no pong")
			}
		case <-conn.Wake():
			for _, frame := range conn.Drain() {
				writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
				err := ws.Write(writeCtx, websocket.MessageText, frame)
				cancel()
				if err != nil {
					conn.Close(websocket.StatusGoingAway, "write failed")
					break
				}
			}
		}
	}
}

// authCache keeps sp_can_subscribe's answers per (user, channel) for authTTL, in process: a client
// that reconnects and resubscribes its channels costs no query.
type authCache struct {
	mu      sync.Mutex
	entries map[string]authEntry
}

type authEntry struct {
	allowed bool
	until   time.Time
}

func newAuthCache() *authCache {
	return &authCache{entries: map[string]authEntry{}}
}

func (a *authCache) allowed(ctx context.Context, session Session, channel string, check SubscribeChecker) (bool, error) {
	key := session.TenantID.String() + "|" + session.UserID.String() + "|" + channel
	now := time.Now()
	a.mu.Lock()
	entry, ok := a.entries[key]
	if len(a.entries) > 10000 {
		for k, e := range a.entries {
			if now.After(e.until) {
				delete(a.entries, k)
			}
		}
	}
	a.mu.Unlock()
	if ok && now.Before(entry.until) {
		return entry.allowed, nil
	}
	allowed, err := check(ctx, session, channel)
	if err != nil {
		return false, err
	}
	a.mu.Lock()
	a.entries[key] = authEntry{allowed: allowed, until: now.Add(authTTL)}
	a.mu.Unlock()
	return allowed, nil
}
