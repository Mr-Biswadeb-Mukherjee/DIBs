// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package redis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingFileCreatesDefaults(t *testing.T) {
	file := filepath.Join(t.TempDir(), "redis.yaml")
	cfg, err := LoadConfig(file)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "127.0.0.1", cfg.Host)
	require.Equal(t, 6379, cfg.Port)
	require.Equal(t, "", cfg.Password)
	_, statErr := os.Stat(file)
	require.NoError(t, statErr)
}

func TestLoadConfig_AppendsMissingKeys(t *testing.T) {
	file := filepath.Join(t.TempDir(), "redis.yaml")
	err := os.WriteFile(file, []byte("host: \"localhost\"\n"), 0o600)
	require.NoError(t, err)

	cfg, err := LoadConfig(file)
	require.NoError(t, err)
	require.Equal(t, "localhost", cfg.Host)
	require.Equal(t, 6379, cfg.Port)

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Contains(t, string(data), "password: \"\"")
	require.Contains(t, string(data), "dial_timeout:")
	require.Contains(t, string(data), "health_tick:")
}

func TestLoadConfig_DefaultTimeoutsApplied(t *testing.T) {
	file := "test-redis.yaml"
	err := os.WriteFile(file, []byte(`
host: localhost
port: 6379
password: ""
db: 0
prefix: test
`), 0o600)
	require.NoError(t, err)
	defer os.Remove(file)

	cfg, err := LoadConfig(file)
	require.NoError(t, err)

	require.Equal(t, 5, cfg.DialTimeout)
	require.Equal(t, 5, cfg.ReadTimeout)
	require.Equal(t, 5, cfg.WriteTimeout)
	require.Equal(t, 10, cfg.HealthTick)
	require.Equal(t, 20, cfg.BackoffMax)
	require.Equal(t, "test:", cfg.Prefix)
}

func TestLoadConfig_PrefixNormalization(t *testing.T) {
	file := "test-prefix.yaml"
	err := os.WriteFile(file, []byte(`
host: localhost
port: 6379
password: ""
prefix: cache
`), 0o600)
	require.NoError(t, err)
	defer os.Remove(file)

	cfg, err := LoadConfig(file)
	require.NoError(t, err)

	require.Equal(t, "cache:", cfg.Prefix)
}

func TestLoadConfig_InvalidHost(t *testing.T) {
	file := "test-badhost.yaml"
	err := os.WriteFile(file, []byte(`
cluster: false
host: ""
port: 6379
password: ""
`), 0o600)
	require.NoError(t, err)
	defer os.Remove(file)

	cfg, err := LoadConfig(file)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestLoadConfig_ClusterModeAllowsMissingHost(t *testing.T) {
	file := "test-cluster.yaml"
	err := os.WriteFile(file, []byte(`
cluster: true
password: ""
addrs:
  - "127.0.0.1:7000"
  - "127.0.0.1:7001"
prefix: redis
`), 0o600)
	require.NoError(t, err)
	defer os.Remove(file)

	cfg, err := LoadConfig(file)
	require.NoError(t, err)

	require.True(t, cfg.Cluster)
	require.Len(t, cfg.Addrs, 2)
	require.Equal(t, "redis:", cfg.Prefix)
}
