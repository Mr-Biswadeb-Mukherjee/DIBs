package redis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/require"
)

// helper to build a minimal config pointing at localhost:6379 (common dev Redis)
func localCfg() *RedisConfig {
	return &RedisConfig{
		Host:         "127.0.0.1",
		Port:         6379,
		Username:     "",
		Password:     "8697575043",
		DB:           0,
		MaxRetries:   1,
		PoolSize:     10,
		MinIdleConns: 1,
		Cluster:      false,
		Prefix:       "test:",
		DialTimeout:  1,
		ReadTimeout:  1,
		WriteTimeout: 1,
		HealthTick:   10,
		BackoffMax:   2,
	}
}

// ensure Redis available; skip test if not (defensive)
func requireRedis(t *testing.T) {
	cfg := localCfg()

	// try connect
	rc, err := NewClient(cfg, nil)
	if err != nil {
		t.Skipf("Redis not available at %s:%d — skipping (err=%v)", cfg.Host, cfg.Port, err)
		return
	}

	// try ping
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = rc.client.Ping(ctx).Err()
	_ = rc.Close() // close once only

	if err != nil {
		t.Skipf("Redis ping failed — skipping (err=%v)", err)
	}
}

func TestSetGetDelIncr_HSetHGet_ExecTx_Eval_Close(t *testing.T) {
	requireRedis(t)

	cfg := localCfg()
	client, err := NewClient(cfg, nil)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	ctx := context.Background()

	// keys used in test (prefixed by client)
	k1 := "unit:key1"
	k2 := "unit:key2"
	hk := "unit:hash"

	// Set/Get
	err = client.SetValue(ctx, k1, "v1", 0)
	require.NoError(t, err)
	got, err := client.GetValue(ctx, k1)
	require.NoError(t, err)
	require.Equal(t, "v1", got)

	// Incr
	_ = client.Delete(ctx, k2) // ensure clean
	n, err := client.Incr(ctx, k2)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	n2, err := client.Incr(ctx, k2)
	require.NoError(t, err)
	require.EqualValues(t, 2, n2)

	// HSet / HGet
	err = client.HSet(ctx, hk, map[string]interface{}{"f1": "fv1"})
	require.NoError(t, err)
	val, err := client.HGet(ctx, hk, "f1")
	require.NoError(t, err)
	require.Equal(t, "fv1", val)

	// ExecTx: set a value inside a transaction
	err = client.ExecTx(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, client.key("txk"), "txv", 0)
		return nil
	})
	require.NoError(t, err)
	vtx, err := client.GetValue(ctx, "txk")
	require.NoError(t, err)
	require.Equal(t, "txv", vtx)

	// Eval: simple Lua that returns ARGV[1]
	out, err := client.Eval(ctx, "return ARGV[1]", []string{}, "arg1")
	require.NoError(t, err)
	require.Equal(t, "arg1", out)

	// cleanup keys
	_ = client.Delete(ctx, k1)
	_ = client.Delete(ctx, k2)
	_ = client.Delete(ctx, hk)
	_ = client.Delete(ctx, "txk")
}
