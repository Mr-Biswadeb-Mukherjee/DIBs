// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

//redis.go

package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

//
// ==========================
//        CLIENT OPS
// ==========================
//

func (r *RedisClient) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, r.key(key), v, ttl).Err()
}

func (r *RedisClient) GetValue(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, r.key(key)).Result()
}

func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.key(key)).Err()
}

func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, r.key(key)).Result()
}

func (r *RedisClient) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	return r.client.HSet(ctx, r.key(key), values).Err()
}

func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, r.key(key), field).Result()
}

func (r *RedisClient) ExecTx(ctx context.Context, fn func(pipe redis.Pipeliner) error) error {
	pipe := r.client.TxPipeline()

	if err := fn(pipe); err != nil {
		pipe.Discard()
		return err
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	realKeys := make([]string, len(keys))
	for i, k := range keys {
		realKeys[i] = r.key(k)
	}
	return r.client.Eval(ctx, script, realKeys, args...).Result()
}

func (r *RedisClient) RPush(ctx context.Context, key string, values ...string) error {
	if len(values) == 0 {
		return nil
	}
	args := make([]interface{}, len(values))
	for i, v := range values {
		args[i] = v
	}
	return r.client.RPush(ctx, r.key(key), args...).Err()
}

func (r *RedisClient) BLPop(ctx context.Context, timeout time.Duration, key string) (string, bool, error) {
	out, err := r.client.BLPop(ctx, timeout, r.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if len(out) < 2 {
		return "", false, nil
	}
	return out[1], true, nil
}

func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, r.key(key), ttl).Err()
}

//
// ==========================
//        GLOBAL CLIENT
// ==========================
//

var global *RedisClient

func Init(configPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	rc, err := NewClient(cfg, silentLogger{})
	if err != nil {
		return err
	}

	global = rc
	return nil
}

func Client() *RedisClient {
	return global
}

//
// ==========================
//        GLOBAL HELPERS
// ==========================
//

func Set(key string, v interface{}) error {
	return global.SetValue(context.Background(), key, v, 0)
}

func SetTTL(key string, v interface{}, ttl time.Duration) error {
	return global.SetValue(context.Background(), key, v, ttl)
}

func Get(key string) (string, error) {
	return global.GetValue(context.Background(), key)
}

func Del(key string) error {
	return global.Delete(context.Background(), key)
}

func Incr(key string) (int64, error) {
	return global.Incr(context.Background(), key)
}

func HSet(key string, values map[string]interface{}) error {
	return global.HSet(context.Background(), key, values)
}

func HGet(key, field string) (string, error) {
	return global.HGet(context.Background(), key, field)
}

func ExecTx(fn func(pipe redis.Pipeliner) error) error {
	return global.ExecTx(context.Background(), fn)
}

func Close() error {
	if global == nil {
		return nil
	}
	return global.Close()
}
