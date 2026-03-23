// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"errors"

	app "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app"
	config "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/config"
	cooldown "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/cooldown"
	logger "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/logger"
	redis "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/redis"
)

type cooldownFactoryAdapter struct{}

func (cooldownFactoryAdapter) New() app.CooldownManager {
	return cooldown.NewManager()
}

func BuildEngineDependencies() (app.Dependencies, error) {
	paths := loadRuntimePaths()
	if err := prepareRuntimeEnvironment(paths); err != nil {
		return app.Dependencies{}, err
	}

	cfg, err := config.LoadOrCreateConfig(paths.settingConf)
	if err != nil {
		return app.Dependencies{}, err
	}

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
			RateLimit:      cfg.RateLimit,
			TimeoutSeconds: cfg.TimeoutSeconds,
			MaxRetries:     cfg.MaxRetries,
			AutoScale:      cfg.AutoScale,
			UpstreamDNS:    cfg.UpstreamDNS,
			BackupDNS:      cfg.BackupDNS,
			DNSRetries:     cfg.DNSRetries,
			DNSTimeoutMS:   cfg.DNSTimeoutMS,
		},
		Paths: app.Paths{
			KeywordsCSV:     paths.keywordsCSV,
			DNSIntelOutput:  paths.dnsIntelOutput,
			GeneratedOutput: paths.generatedOutput,
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
