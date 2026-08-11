package kernel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type CacheManager struct {
	logger *zap.Logger
	client *redis.Client
}

func NewCacheManager(client *redis.Client, logger *zap.Logger) *CacheManager {
	return &CacheManager{
		logger: logger.Named("cache"),
		client: client,
	}
}

func (cm *CacheManager) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := cm.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return cm.client.Set(ctx, key, data, ttl).Err()
}

func (cm *CacheManager) Delete(ctx context.Context, keys ...string) error {
	return cm.client.Del(ctx, keys...).Err()
}

func (cm *CacheManager) Exists(ctx context.Context, key string) (bool, error) {
	n, err := cm.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (cm *CacheManager) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return cm.client.TTL(ctx, key).Result()
}

func (cm *CacheManager) Increment(ctx context.Context, key string) (int64, error) {
	return cm.client.Incr(ctx, key).Result()
}

func (cm *CacheManager) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return cm.client.IncrBy(ctx, key, value).Result()
}

func (cm *CacheManager) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return cm.client.Expire(ctx, key, ttl).Err()
}

func (cm *CacheManager) Keys(ctx context.Context, pattern string) ([]string, error) {
	return cm.client.Keys(ctx, pattern).Result()
}

func (cm *CacheManager) FlushDB(ctx context.Context) error {
	return cm.client.FlushDB(ctx).Err()
}

func (cm *CacheManager) Health(ctx context.Context) error {
	return cm.client.Ping(ctx).Err()
}