// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	redis "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/redis"
)

const (
	bootstrapTimeout  = 3 * time.Minute
	redisDialTimeout  = 1200 * time.Millisecond
	redisRetryDelay   = 750 * time.Millisecond
	redisRetryAttempt = 10
)

func prepareRuntimeEnvironment(paths runtimePaths) error {
	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := ensureGoModules(ctx, paths.repo, runSetupCommand, commandExists); err != nil {
		return err
	}
	return ensureRedisPrepared(ctx, paths.redisConf, runSetupCommand, commandExists)
}

func ensureGoModules(
	ctx context.Context,
	repo string,
	run commandRunner,
	exists commandLocator,
) error {
	if _, err := exists("go"); err != nil {
		return nil
	}
	modPath := filepath.Join(repo, "go.mod")
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("go module check failed: %w", err)
	}
	if err := run(ctx, repo, "go", "mod", "download"); err != nil {
		return fmt.Errorf("go module setup failed: %w", err)
	}
	return nil
}

func ensureRedisPrepared(
	ctx context.Context,
	redisConf string,
	run commandRunner,
	exists commandLocator,
) error {
	cfg, err := redis.LoadConfig(redisConf)
	if err != nil {
		return fmt.Errorf("redis config setup failed: %w", err)
	}
	if cfg.Cluster || !isLocalRedisHost(cfg.Host) {
		return nil
	}
	if waitRedisReachable(ctx, cfg.Host, cfg.Port) {
		return nil
	}
	if err := installRedisIfMissing(ctx, run, exists); err != nil {
		return err
	}
	if waitRedisReachable(ctx, cfg.Host, cfg.Port) {
		return nil
	}
	if err := startRedisService(ctx, run, exists); err != nil {
		return err
	}
	if waitRedisReachable(ctx, cfg.Host, cfg.Port) {
		return nil
	}
	return fmt.Errorf("redis setup failed: %s:%d is unreachable", cfg.Host, cfg.Port)
}

func isLocalRedisHost(host string) bool {
	clean := strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	switch clean {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func waitRedisReachable(ctx context.Context, host string, port int) bool {
	for i := 0; i < redisRetryAttempt; i++ {
		if redisEndpointReachable(ctx, host, port) {
			return true
		}
		if !waitOrCancel(ctx, redisRetryDelay) {
			return false
		}
	}
	return false
}

func redisEndpointReachable(ctx context.Context, host string, port int) bool {
	for _, candidate := range redisHostCandidates(host) {
		if canDialRedis(ctx, candidate, port) {
			return true
		}
	}
	return false
}

func canDialRedis(ctx context.Context, host string, port int) bool {
	dialCtx, cancel := context.WithTimeout(ctx, redisDialTimeout)
	defer cancel()

	d := net.Dialer{Timeout: redisDialTimeout}
	conn, err := d.DialContext(dialCtx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func redisHostCandidates(host string) []string {
	clean := strings.Trim(host, "[]")
	if clean == "" || strings.EqualFold(clean, "localhost") {
		return []string{"127.0.0.1", "::1"}
	}
	return []string{clean}
}

func waitOrCancel(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
