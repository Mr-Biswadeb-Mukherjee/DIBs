// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package redis

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type redisConfigEntry struct {
	key   string
	value string
}

var redisDefaultConfig = RedisConfig{
	Host:         "127.0.0.1",
	Port:         6379,
	Username:     "",
	Password:     "",
	DB:           0,
	MaxRetries:   3,
	PoolSize:     20,
	MinIdleConns: 5,
	Cluster:      false,
	Prefix:       "",
	DialTimeout:  5,
	ReadTimeout:  5,
	WriteTimeout: 5,
	HealthTick:   10,
	BackoffMax:   20,
}

var redisEntries = []redisConfigEntry{
	{key: "host", value: "\"127.0.0.1\""},
	{key: "port", value: "6379"},
	{key: "username", value: "\"\""},
	{key: "password", value: "\"\""},
	{key: "db", value: "0"},
	{key: "max_retries", value: "3"},
	{key: "pool_size", value: "20"},
	{key: "min_idle_conns", value: "5"},
	{key: "cluster", value: "false"},
	{key: "addrs", value: "[]"},
	{key: "prefix", value: "\"\""},
	{key: "dial_timeout", value: "5"},
	{key: "read_timeout", value: "5"},
	{key: "write_timeout", value: "5"},
	{key: "health_tick", value: "10"},
	{key: "backoff_max", value: "20"},
}

func LoadConfig(file string) (*RedisConfig, error) {
	cleanPath, err := sanitizeRedisConfigPath(file)
	if err != nil {
		return nil, err
	}
	if err := ensureRedisConfigFile(cleanPath); err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(cleanPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read redis config: %w", err)
	}

	cfg := redisDefaultConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("redis config parse error: %w", err)
	}
	normalizeRedisConfig(&cfg)
	if !cfg.Cluster && strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("invalid redis config: missing host")
	}
	return &cfg, nil
}

func ensureRedisConfigFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return writeRedisDefaults(path)
	} else if err != nil {
		return err
	}
	return ensureRedisEntries(path)
}

func writeRedisDefaults(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(redisDefaultConfigText())
	return err
}

func redisDefaultConfigText() string {
	var b strings.Builder
	for _, entry := range redisEntries {
		b.WriteString(entry.key)
		b.WriteString(": ")
		b.WriteString(entry.value)
		b.WriteString("\n")
	}
	return b.String()
}

func ensureRedisEntries(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	existing := redisExistingKeys(content)
	missing := make([]redisConfigEntry, 0, len(redisEntries))
	for _, entry := range redisEntries {
		if _, ok := existing[entry.key]; ok {
			continue
		}
		missing = append(missing, entry)
	}
	if len(missing) == 0 {
		return nil
	}
	return appendRedisEntries(path, missing)
}

func redisExistingKeys(content []byte) map[string]struct{} {
	keys := make(map[string]struct{}, len(redisEntries))
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func appendRedisEntries(path string, entries []redisConfigEntry) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	var b strings.Builder
	b.WriteString("\n# Auto-injected defaults\n")
	for _, entry := range entries {
		b.WriteString(entry.key)
		b.WriteString(": ")
		b.WriteString(entry.value)
		b.WriteString("\n")
	}
	_, err = file.WriteString(b.String())
	return err
}

func sanitizeRedisConfigPath(path string) (string, error) {
	raw := strings.TrimSpace(path)
	if raw == "" {
		return "", errors.New("redis config path is empty")
	}
	cleanPath := filepath.Clean(raw)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return "", fmt.Errorf("redis config path is a directory: %s", cleanPath)
	}
	return cleanPath, nil
}

func normalizeRedisConfig(cfg *RedisConfig) {
	if cfg.Port == 0 {
		cfg.Port = redisDefaultConfig.Port
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = redisDefaultConfig.MaxRetries
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = redisDefaultConfig.PoolSize
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = redisDefaultConfig.MinIdleConns
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = redisDefaultConfig.DialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = redisDefaultConfig.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = redisDefaultConfig.WriteTimeout
	}
	if cfg.HealthTick == 0 {
		cfg.HealthTick = redisDefaultConfig.HealthTick
	}
	if cfg.BackoffMax == 0 {
		cfg.BackoffMax = redisDefaultConfig.BackoffMax
	}
	if cfg.Prefix != "" && cfg.Prefix[len(cfg.Prefix)-1] != ':' {
		cfg.Prefix += ":"
	}
}
