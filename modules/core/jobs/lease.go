package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Renew and release only touch the key while it still holds this lease's token: a holder that
// stalled past its TTL must not extend or delete the lease another instance has since taken.
var (
	renewScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0`)
	releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)
)

// Lease is a single-holder lock with a TTL: SET NX PX to take it, a token-checked PEXPIRE to keep it.
// A holder that dies simply stops renewing and the lease frees itself after one TTL.
type Lease struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

func NewLease(client *redis.Client, key string, ttl time.Duration) *Lease {
	return &Lease{client: client, key: key, token: uuid.NewString(), ttl: ttl}
}

// Acquire takes the lease if nobody holds it.
func (l *Lease) Acquire(ctx context.Context) (bool, error) {
	err := l.client.SetArgs(ctx, l.key, l.token, redis.SetArgs{Mode: "NX", TTL: l.ttl}).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return err == nil, err
}

// Renew extends the lease, answering false when it is no longer ours.
func (l *Lease) Renew(ctx context.Context) (bool, error) {
	n, err := renewScript.Run(ctx, l.client, []string{l.key}, l.token, l.ttl.Milliseconds()).Int()
	return n == 1, err
}

// Release frees the lease now instead of after its TTL, so a clean shutdown hands over at once.
func (l *Lease) Release(ctx context.Context) error {
	return releaseScript.Run(ctx, l.client, []string{l.key}, l.token).Err()
}
