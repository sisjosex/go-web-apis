package services

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// ValkeyService is the process's one Valkey client (INFRA-001): the rate limit, the job client, the
// worker, the scheduler and asynqmon all share its pool. Nothing it holds is the only copy of
// business data, so its absence is never fatal to the api role.
type ValkeyService interface {
	// Client is the shared go-redis client, for the libraries that take one.
	Client() *redis.Client
	// Ping answers whether Valkey is reachable right now.
	Ping(ctx context.Context) error
	Close() error
}

type valkeyService struct {
	client *redis.Client
}

// NewValkeyService parses REDIS_URL and builds the client without dialing: go-redis connects on the
// first command, so a Valkey that is down at boot costs a failed command later, not a failed start.
// Without a URL it answers nil — callers read nil as "no Valkey", and the api role runs as before.
func NewValkeyService(redisURL string) (ValkeyService, error) {
	if redisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL: %w", err)
	}
	log.Printf("Valkey client configured (%s, db %d)", opt.Addr, opt.DB)
	return &valkeyService{client: redis.NewClient(opt)}, nil
}

func (vs *valkeyService) Client() *redis.Client {
	return vs.client
}

func (vs *valkeyService) Ping(ctx context.Context) error {
	return vs.client.Ping(ctx).Err()
}

func (vs *valkeyService) Close() error {
	return vs.client.Close()
}
