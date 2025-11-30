//redis.go

package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

//
// ==========================
//       CONFIG LOADING
// ==========================
//

func LoadConfig(file string) (*RedisConfig, error) {
	viper.SetConfigFile(file)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("warning: cannot read redis config file: %v", err)
	}

	var cfg RedisConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("redis config parse error: %w", err)
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 5
	}
	if cfg.HealthTick == 0 {
		cfg.HealthTick = 10
	}
	if cfg.BackoffMax == 0 {
		cfg.BackoffMax = 20
	}

	if cfg.Prefix != "" && cfg.Prefix[len(cfg.Prefix)-1] != ':' {
		cfg.Prefix += ":"
	}

	if !cfg.Cluster && cfg.Host == "" {
		return nil, errors.New("invalid redis config: missing host")
	}

	return &cfg, nil
}

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

//
// ==========================
//        GLOBAL CLIENT
// ==========================
//

var global *RedisClient

func Init() error {
	cfg, err := LoadConfig("Setting/redis.yaml")
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
