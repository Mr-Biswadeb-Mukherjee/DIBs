//redis_utils.go

package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

//
// ==========================
//        CONFIG STRUCT
// ==========================
//

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	MaxRetries   int    `mapstructure:"max_retries"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`

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
//         INIT CLIENT
// ==========================
//

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

		r.logger.Errorf("Redis ping failed: %v. Retrying in %s", err, backoff)
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
//     KEY PREFIX HANDLING
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
//           CLOSE
// ==========================
//

func (r *RedisClient) Close() error {
	r.mu.Lock()
	if r.stopChan == nil {
		r.mu.Unlock()
		return nil
	}
	close(r.stopChan)
	r.stopChan = nil
	r.mu.Unlock()

	r.wg.Wait()
	return r.client.Close()
}
