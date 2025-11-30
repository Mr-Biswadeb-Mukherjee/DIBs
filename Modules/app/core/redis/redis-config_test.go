package redis

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test case updated: missing file SHOULD return an error
func TestLoadConfig_MissingFileReturnsError(t *testing.T) {
	cfg, err := LoadConfig("nonexistent.yaml")
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestLoadConfig_DefaultTimeoutsApplied(t *testing.T) {
	file := "test-redis.yaml"
	err := os.WriteFile(file, []byte(`
host: localhost
port: 6379
password: "8697575043"
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
password: "8697575043"
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
password: "8697575043"
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
password: "8697575043"
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
