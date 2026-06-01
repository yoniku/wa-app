package app

import (
	"context"
	"fmt"
	"time"

	"github.com/byte-v-forge/common-lib/redisx"
)

type RedisRuntime struct {
	clientClose func() error
	idempotency *redisx.TTLFlagStore
	transient   *redisx.StringStore
	sessions    *redisx.TTLFlagStore
}

func NewRedisRuntime(ctx context.Context, url string) (*RedisRuntime, error) {
	client, err := redisx.NewRequiredClient(ctx, url, "PLATFORM_REDIS_URL is required")
	if err != nil {
		return nil, err
	}
	return &RedisRuntime{
		clientClose: client.Close,
		idempotency: redisx.NewTTLFlagStore(client, "wa-app:idempotency", 10*time.Minute, "1"),
		transient:   redisx.NewStringStore(client, "wa-app:transient-state", 30*time.Minute),
		sessions:    redisx.NewTTLFlagStore(client, "wa-app:message-session", 5*time.Minute, "open"),
	}, nil
}

func (r *RedisRuntime) Close() error {
	if r == nil || r.clientClose == nil {
		return nil
	}
	return r.clientClose()
}

func (r *RedisRuntime) ClaimRequest(ctx context.Context, requestID string, ttl time.Duration) (bool, error) {
	if requestID == "" {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return r.idempotency.Claim(ctx, requestID, ttl)
}

func (r *RedisRuntime) SaveTransientState(ctx context.Context, ref string, data []byte, ttl time.Duration) error {
	if ref == "" {
		return fmt.Errorf("transient state ref is required")
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return r.transient.SaveTTL(ctx, ref, string(data), ttl)
}

func (r *RedisRuntime) GetTransientState(ctx context.Context, ref string) ([]byte, error) {
	if ref == "" {
		return nil, fmt.Errorf("transient state ref is required")
	}
	data, found, err := r.transient.Load(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("transient state ref not found")
	}
	return []byte(data), nil
}

func (r *RedisRuntime) DeleteTransientState(ctx context.Context, ref string) error {
	if ref == "" {
		return nil
	}
	return r.transient.Delete(ctx, ref)
}

func (r *RedisRuntime) OpenSessionLease(ctx context.Context, sessionID string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return r.sessions.Save(ctx, sessionID, ttl)
}

func (r *RedisRuntime) CloseSessionLease(ctx context.Context, sessionID string) error {
	return r.sessions.Delete(ctx, sessionID)
}
