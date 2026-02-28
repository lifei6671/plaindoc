package captchastore

import (
	"context"
	"errors"
	"strings"
	"time"
)

const defaultRedisKeyPrefix = "captcha"

// RedisClient 定义 Redis 扩展的最小能力。
type RedisClient interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, bool, error)
	Del(ctx context.Context, key string) error
}

// RedisBackendOptions 描述 Redis 后端参数。
type RedisBackendOptions struct {
	KeyPrefix string
	NowFunc   func() time.Time
}

// RedisBackend 是 Redis 后端实现，适用于多实例共享验证码状态。
type RedisBackend struct {
	client    RedisClient
	keyPrefix string
	nowFn     func() time.Time
}

// NewRedisBackend 创建 Redis 后端。
func NewRedisBackend(client RedisClient, options RedisBackendOptions) (*RedisBackend, error) {
	if client == nil {
		return nil, errors.New("captcha redis client is required")
	}
	keyPrefix := strings.TrimSpace(options.KeyPrefix)
	if keyPrefix == "" {
		keyPrefix = defaultRedisKeyPrefix
	}
	nowFn := options.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &RedisBackend{
		client:    client,
		keyPrefix: keyPrefix,
		nowFn:     nowFn,
	}, nil
}

// Set 写入 Redis。
func (b *RedisBackend) Set(ctx context.Context, record Record) error {
	if b == nil || b.client == nil {
		return errors.New("captcha redis backend is not initialized")
	}

	ttl := time.Duration(0)
	if !record.ExpiresAt.IsZero() {
		ttl = record.ExpiresAt.Sub(b.currentTime())
		if ttl <= 0 {
			return b.client.Del(ctx, b.key(record.ID))
		}
	}
	return b.client.Set(ctx, b.key(record.ID), record.Value, ttl)
}

// Get 读取 Redis。
func (b *RedisBackend) Get(ctx context.Context, id string) (Record, bool, error) {
	if b == nil || b.client == nil {
		return Record{}, false, errors.New("captcha redis backend is not initialized")
	}
	value, found, err := b.client.Get(ctx, b.key(id))
	if err != nil {
		return Record{}, false, err
	}
	if !found {
		return Record{}, false, nil
	}
	return Record{
		ID:    id,
		Value: value,
	}, true, nil
}

// Delete 删除 Redis 记录。
func (b *RedisBackend) Delete(ctx context.Context, id string) error {
	if b == nil || b.client == nil {
		return errors.New("captcha redis backend is not initialized")
	}
	return b.client.Del(ctx, b.key(id))
}

func (b *RedisBackend) key(id string) string {
	normalizedID := strings.TrimSpace(id)
	return b.keyPrefix + ":" + normalizedID
}

func (b *RedisBackend) currentTime() time.Time {
	if b == nil || b.nowFn == nil {
		return time.Now().UTC()
	}
	return b.nowFn().UTC()
}
