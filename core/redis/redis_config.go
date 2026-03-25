// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package redis

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

var redisEntries = buildRedisEntries(redisDefaultConfig)

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
	return writeRedisEntries(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, "", redisEntries)
}

func ensureRedisEntries(path string) error {
	content, err := readRedisConfigContent(path)
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
	return writeRedisEntries(path, os.O_APPEND|os.O_WRONLY, "\n# Auto-injected defaults\n", entries)
}

func writeRedisEntries(
	path string,
	flags int,
	header string,
	entries []redisConfigEntry,
) error {
	cleanPath, err := sanitizeRedisConfigPath(path)
	if err != nil {
		return err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeRedisConfigPath.
	file, err := os.OpenFile(cleanPath, flags, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(renderRedisEntries(header, entries))
	return err
}

func readRedisConfigContent(path string) ([]byte, error) {
	cleanPath, err := sanitizeRedisConfigPath(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeRedisConfigPath.
	return os.ReadFile(cleanPath)
}

func renderRedisEntries(header string, entries []redisConfigEntry) string {
	var b strings.Builder
	b.WriteString(header)
	for _, entry := range entries {
		b.WriteString(entry.key)
		b.WriteString(": ")
		b.WriteString(entry.value)
		b.WriteString("\n")
	}
	return b.String()
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
	applyRedisIntDefaults(
		intDefault{target: &cfg.Port, fallback: redisDefaultConfig.Port},
		intDefault{target: &cfg.MaxRetries, fallback: redisDefaultConfig.MaxRetries},
		intDefault{target: &cfg.PoolSize, fallback: redisDefaultConfig.PoolSize},
		intDefault{target: &cfg.MinIdleConns, fallback: redisDefaultConfig.MinIdleConns},
		intDefault{target: &cfg.DialTimeout, fallback: redisDefaultConfig.DialTimeout},
		intDefault{target: &cfg.ReadTimeout, fallback: redisDefaultConfig.ReadTimeout},
		intDefault{target: &cfg.WriteTimeout, fallback: redisDefaultConfig.WriteTimeout},
		intDefault{target: &cfg.HealthTick, fallback: redisDefaultConfig.HealthTick},
		intDefault{target: &cfg.BackoffMax, fallback: redisDefaultConfig.BackoffMax},
	)
	if strings.HasSuffix(cfg.Prefix, ":") || cfg.Prefix == "" {
		return
	}
	cfg.Prefix += ":"
}

type intDefault struct {
	target   *int
	fallback int
}

func applyRedisIntDefaults(defaults ...intDefault) {
	for _, item := range defaults {
		if *item.target == 0 {
			*item.target = item.fallback
		}
	}
}

func buildRedisEntries(cfg RedisConfig) []redisConfigEntry {
	return []redisConfigEntry{
		redisStringEntry("host", cfg.Host),
		redisIntEntry("port", cfg.Port),
		redisStringEntry("username", cfg.Username),
		redisStringEntry("password", cfg.Password),
		redisIntEntry("db", cfg.DB),
		redisIntEntry("max_retries", cfg.MaxRetries),
		redisIntEntry("pool_size", cfg.PoolSize),
		redisIntEntry("min_idle_conns", cfg.MinIdleConns),
		redisBoolEntry("cluster", cfg.Cluster),
		{key: "addrs", value: "[]"},
		redisStringEntry("prefix", cfg.Prefix),
		redisIntEntry("dial_timeout", cfg.DialTimeout),
		redisIntEntry("read_timeout", cfg.ReadTimeout),
		redisIntEntry("write_timeout", cfg.WriteTimeout),
		redisIntEntry("health_tick", cfg.HealthTick),
		redisIntEntry("backoff_max", cfg.BackoffMax),
	}
}

func redisStringEntry(key, value string) redisConfigEntry {
	return redisConfigEntry{key: key, value: strconv.Quote(value)}
}
func redisIntEntry(key string, value int) redisConfigEntry {
	return redisConfigEntry{key: key, value: strconv.Itoa(value)}
}
func redisBoolEntry(key string, value bool) redisConfigEntry {
	return redisConfigEntry{key: key, value: strconv.FormatBool(value)}
}
