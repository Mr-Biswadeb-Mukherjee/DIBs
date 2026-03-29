// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"errors"
	"time"

	app "github.com/Mr-Biswadeb-Mukherjee/DIBs/Engine/app"
	config "github.com/Mr-Biswadeb-Mukherjee/DIBs/core/config"
	cooldown "github.com/Mr-Biswadeb-Mukherjee/DIBs/core/cooldown"
	logger "github.com/Mr-Biswadeb-Mukherjee/DIBs/core/logger"
	redis "github.com/Mr-Biswadeb-Mukherjee/DIBs/core/redis"
)

type cooldownFactoryAdapter struct{}

func (cooldownFactoryAdapter) New() app.CooldownManager {
	return cooldown.NewManager()
}

func BuildEngineDependencies() (app.Dependencies, error) {
	paths := loadRuntimePaths()
	if err := ensureRuntimeAssets(paths); err != nil {
		return app.Dependencies{}, err
	}
	if err := prepareRuntimeEnvironment(paths); err != nil {
		return app.Dependencies{}, err
	}

	cfg, err := config.LoadOrCreateConfig(paths.settingConf)
	if err != nil {
		return app.Dependencies{}, err
	}
	paths = alignRuntimeOutputPaths(paths, cfg.ResolveIntervalHours, time.Now())

	logs := app.LogSet{
		App:         logger.NewLoggerInDir("app", paths.logsDir),
		DNS:         logger.NewLoggerInDir("dns", paths.logsDir),
		RateLimiter: logger.NewLoggerInDir("ratelimiter", paths.logsDir),
	}

	if err := redis.Init(paths.redisConf); err != nil {
		closeLogs(logs)
		return app.Dependencies{}, err
	}

	cache := redis.Client()
	if cache == nil {
		closeLogs(logs)
		_ = redis.Close()
		return app.Dependencies{}, errors.New("redis client unavailable")
	}

	return app.Dependencies{
		Config: app.Config{
			RateLimit:            cfg.RateLimit,
			RateLimitCeiling:     cfg.RateLimitCeiling,
			CooldownAfter:        cfg.CooldownAfter,
			CooldownDuration:     cfg.CooldownDuration,
			TimeoutSeconds:       cfg.TimeoutSeconds,
			MaxRetries:           cfg.MaxRetries,
			AutoScale:            cfg.AutoScale,
			UpstreamDNS:          cfg.UpstreamDNS,
			BackupDNS:            cfg.BackupDNS,
			DNSRetries:           cfg.DNSRetries,
			DNSTimeoutMS:         cfg.DNSTimeoutMS,
			ResolveIntervalHours: cfg.ResolveIntervalHours,
		},
		Paths: app.Paths{
			KeywordsCSV:      paths.keywordsCSV,
			DNSIntelOutput:   paths.dnsIntelOutput,
			GeneratedOutput:  paths.generatedOutput,
			ResolvedOutput:   paths.resolvedOutput,
			ClusterOutput:    paths.clusterOutput,
			RunMetricsOutput: paths.runMetricsOutput,
		},
		Startup:     newStartupAdapter(),
		Logs:        logs,
		Cache:       cache,
		Limiter:     rateLimiterAdapter{},
		WorkerPools: workerPoolFactoryAdapter{},
		Cooldowns:   cooldownFactoryAdapter{},
		Adaptive:    adaptiveFactoryAdapter{},
		Writers:     writerFactoryAdapter{},
	}, nil
}

func closeLogs(logs app.LogSet) {
	_ = logs.App.Close()
	_ = logs.DNS.Close()
	_ = logs.RateLimiter.Close()
}
