package gatewaysign

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// RedisNonceStore implements NonceStore on top of go-zero's redis client.
type RedisNonceStore struct {
	Client    *redis.Redis
	KeyPrefix string
}

// NewRedisNonceStore returns a NonceStore using the supplied redis client.
// keyPrefix is prepended to every nonce; use a stable namespace such as
// "gw:nonce:" to keep signed requests separate from other Redis traffic.
func NewRedisNonceStore(client *redis.Redis, keyPrefix string) *RedisNonceStore {
	if keyPrefix == "" {
		keyPrefix = "gw:nonce:"
	}
	return &RedisNonceStore{Client: client, KeyPrefix: keyPrefix}
}

// Reserve marks the nonce as seen for ttl. Returns true on first sighting.
func (s *RedisNonceStore) Reserve(ctx context.Context, nonce string, ttl time.Duration) (bool, error) {
	if s == nil || s.Client == nil {
		return false, fmt.Errorf("gatewaysign: redis nonce store not initialised")
	}
	key := s.KeyPrefix + nonce
	ok, err := s.Client.SetnxExCtx(ctx, key, "1", int(ttl.Seconds()))
	if err != nil {
		return false, fmt.Errorf("gatewaysign: redis setnx: %w", err)
	}
	return ok, nil
}
