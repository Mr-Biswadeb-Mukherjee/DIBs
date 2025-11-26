package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

//
// ==========================
//        CONFIG STRUCT
// ==========================
//

type RedisConfig struct {
	Host         string   `mapstructure:"host"`
	Port         int      `mapstructure:"port"`
	Username     string   `mapstructure:"username"`
	Password     string   `mapstructure:"password"`
	DB           int      `mapstructure:"db"`
	MaxRetries   int      `mapstructure:"max_retries"`
	PoolSize     int      `mapstructure:"pool_size"`
	MinIdleConns int      `mapstructure:"min_idle_conns"`

	Cluster bool     `mapstructure:"cluster"`
	Addrs   []string `mapstructure:"addrs"`

	Prefix string `mapstructure:"prefix"`

	DialTimeout  int `mapstructure:"dial_timeout"`
	ReadTimeout  int `mapstructure:"read_timeout"`
	WriteTimeout int `mapstructure:"write_timeout"`
	HealthTick   int `mapstructure:"health_tick"`
	BackoffMax   int `mapstructure:"backoff_max"`
}

//
// ==========================
//       LOGGER INTERFACE
// ==========================
//

type Logger interface {
	Infof(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

type silentLogger struct{}

func (silentLogger) Infof(string, ...interface{})  {}
func (silentLogger) Errorf(string, ...interface{}) {}

//
// ==========================
//       INTERNAL CLIENT
// ==========================
//

type RedisClient struct {
	mu       sync.RWMutex
	client   redis.UniversalClient
	cfg      *RedisConfig
	logger   Logger
	prefix   string
	stopChan chan struct{}
	wg       sync.WaitGroup
}

//
// ==========================
//            INIT
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

	// defaults
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

	// prefix normalization
	if cfg.Prefix != "" && !strings.HasSuffix(cfg.Prefix, ":") {
		cfg.Prefix += ":"
	}

	// validation
	if !cfg.Cluster && cfg.Host == "" {
		return nil, errors.New("redis config invalid: host missing for non-cluster mode")
	}

	return &cfg, nil
}

func NewClient(cfg *RedisConfig, logger Logger) (*RedisClient, error) {
	if logger == nil {
		logger = silentLogger{}
	}

	r := &RedisClient{
		cfg:      cfg,
		logger:   logger,
		prefix:   cfg.Prefix,
		stopChan: make(chan struct{}),
	}

	var client redis.UniversalClient

	if cfg.Cluster {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.Addrs,
			Username:     cfg.Username,
			Password:     cfg.Password,
			MaxRetries:   cfg.MaxRetries,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Username:     cfg.Username,
			Password:     cfg.Password,
			DB:           cfg.DB,
			MaxRetries:   cfg.MaxRetries,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		})
	}

	r.client = client

	if err := r.waitForReady(context.Background()); err != nil {
		return nil, err
	}

	r.startHealthChecker()

	logger.Infof("Redis initialized. Prefix=%q", cfg.Prefix)
	return r, nil
}

//
// ==========================
//        HEALTH CHECKING
// ==========================
//

func (r *RedisClient) waitForReady(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := time.Duration(r.cfg.BackoffMax) * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := r.client.Ping(ctx).Result()
		if err == nil {
			return nil
		}

		r.logger.Errorf("Redis ping failed: %v. Retry in %s", err, backoff)
		time.Sleep(backoff)

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (r *RedisClient) startHealthChecker() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(time.Duration(r.cfg.HealthTick) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.stopChan:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_, err := r.client.Ping(ctx).Result()
				cancel()

				if err != nil {
					r.logger.Errorf("Redis unhealthy: %v", err)
					_ = r.waitForReady(context.Background())
				}
			}
		}
	}()
}

//
// ==========================
//          API HELPERS
// ==========================
//

func (r *RedisClient) key(k string) string {
	if r.prefix == "" {
		return k
	}
	return r.prefix + k
}

//
// ==========================
//          PUBLIC API
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

func Client() *RedisClient {
    return global
}


func (r *RedisClient) Close() error {
	close(r.stopChan)
	r.wg.Wait()
	return r.client.Close()
}

//
// ==========================
//     OPTIONAL GLOBAL API
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

func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {

    // Apply prefix to each key
    realKeys := make([]string, len(keys))
    for i, k := range keys {
        realKeys[i] = r.key(k)
    }

    // Eval requires args as ...interface{}
    return r.client.Eval(ctx, script, realKeys, args...).Result()
}


func Close() error {
	if global == nil {
		return nil
	}
	return global.Close()
}
