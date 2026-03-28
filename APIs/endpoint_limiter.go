// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	redis "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/redis"
)

const (
	apiRateLimiterPrefix = "api:route"
	redisConfigName      = "redis.yaml"
	redisOpTimeout       = 2 * time.Second
)

type EndpointRateLimiter struct {
	initOnce sync.Once
	initErr  error
}

func NewEndpointRateLimiter() *EndpointRateLimiter {
	return &EndpointRateLimiter{}
}

func (l *EndpointRateLimiter) Allow(
	ctx context.Context,
	routeName string,
	perSec int,
	perMin int,
) (bool, error) {
	if err := l.ensureClient(); err != nil {
		return false, err
	}
	client := redis.Client()
	if client == nil {
		return false, fmt.Errorf("redis client unavailable")
	}
	if ok, err := l.allowWindow(ctx, client, routeName, "s", perSec, time.Second); !ok || err != nil {
		return ok, err
	}
	return l.allowWindow(ctx, client, routeName, "m", perMin, time.Minute)
}

func (l *EndpointRateLimiter) ensureClient() error {
	l.initOnce.Do(func() {
		if redis.Client() != nil {
			return
		}
		path, err := normalizeSettingFilePath(filepath.Join(settingDirName, redisConfigName))
		if err != nil {
			l.initErr = err
			return
		}
		l.initErr = redis.Init(path)
	})
	return l.initErr
}

func (l *EndpointRateLimiter) allowWindow(
	ctx context.Context,
	client *redis.RedisClient,
	routeName string,
	window string,
	limit int,
	ttl time.Duration,
) (bool, error) {
	route := sanitizeLimiterToken(routeName)
	stamp := time.Now().UTC()
	key := buildRateKey(route, window, stamp)
	opCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	next, err := client.Incr(opCtx, key)
	if err != nil {
		return false, err
	}
	if next == 1 {
		if err := client.Expire(opCtx, key, ttl); err != nil {
			return false, err
		}
	}
	return next <= int64(limit), nil
}

func buildRateKey(routeName, window string, stamp time.Time) string {
	if window == "m" {
		return fmt.Sprintf("%s:%s:m:%s", apiRateLimiterPrefix, routeName, stamp.Format("200601021504"))
	}
	return fmt.Sprintf("%s:%s:s:%s", apiRateLimiterPrefix, routeName, stamp.Format("20060102150405"))
}

func sanitizeLimiterToken(value string) string {
	clean := strings.ToLower(strings.TrimSpace(value))
	if clean == "" {
		return "unknown"
	}
	return strings.NewReplacer(" ", "_", "/", "_", ":", "_").Replace(clean)
}
