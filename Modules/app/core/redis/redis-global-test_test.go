package redis

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

//
// mockGlobalClient – only tracks calls needed for testing global helpers
//

type mockGlobalClient struct {
	setCalls  int32
	getCalls  int32
	delCalls  int32
	incrCalls int32
	hsetCalls int32
	hgetCalls int32

	val  string
	nval int64
	hval string
}

func (m *mockGlobalClient) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) *redis.StatusCmd {
	atomic.AddInt32(&m.setCalls, 1)
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func (m *mockGlobalClient) Get(ctx context.Context, key string) *redis.StringCmd {
	atomic.AddInt32(&m.getCalls, 1)
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(m.val)
	return cmd
}

func (m *mockGlobalClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	atomic.AddInt32(&m.delCalls, 1)
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func (m *mockGlobalClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	atomic.AddInt32(&m.incrCalls, 1)
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(m.nval)
	return cmd
}

func (m *mockGlobalClient) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	atomic.AddInt32(&m.hsetCalls, 1)
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func (m *mockGlobalClient) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	atomic.AddInt32(&m.hgetCalls, 1)
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(m.hval)
	return cmd
}

//
// mockAdapter – satisfies the FULL redis.UniversalClient interface
// by embedding a real redis.Client and overriding only needed methods
//

type mockAdapter struct {
	*redis.Client
	m *mockGlobalClient
}

// overrides for the methods RedisClient actually uses

func (a *mockAdapter) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) *redis.StatusCmd {
	return a.m.Set(ctx, key, val, ttl)
}

func (a *mockAdapter) Get(ctx context.Context, key string) *redis.StringCmd {
	return a.m.Get(ctx, key)
}

func (a *mockAdapter) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return a.m.Del(ctx, keys...)
}

func (a *mockAdapter) Incr(ctx context.Context, key string) *redis.IntCmd {
	return a.m.Incr(ctx, key)
}

func (a *mockAdapter) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return a.m.HSet(ctx, key, values...)
}

func (a *mockAdapter) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return a.m.HGet(ctx, key, field)
}

//
// Test global helpers using wrapped mock
//

func TestGlobalHelpers(t *testing.T) {
	// restore original global after test
	old := global
	t.Cleanup(func() { global = old })

	mock := &mockGlobalClient{
		val:  "value",
		nval: 42,
		hval: "hvalue",
	}

	// adapter wraps mock + embeds real client
	adapter := &mockAdapter{
		Client: redis.NewClient(&redis.Options{
			Addr: "127.0.0.1:6379", // never used, but needed for complete struct
		}),
		m: mock,
	}

	global = &RedisClient{
		client: adapter,
		cfg:    &RedisConfig{},
		logger: silentLogger{},
	}

	// Set
	require.NoError(t, Set("a", "x"))
	require.EqualValues(t, 1, mock.setCalls)

	// Get
	v, err := Get("a")
	require.NoError(t, err)
	require.Equal(t, "value", v)

	// Del
	require.NoError(t, Del("a"))
	require.EqualValues(t, 1, mock.delCalls)

	// Incr
	n, err := Incr("n")
	require.NoError(t, err)
	require.EqualValues(t, 42, n)

	// HSet
	require.NoError(t, HSet("h", map[string]interface{}{"f": "v"}))
	require.EqualValues(t, 1, mock.hsetCalls)

	// HGet
	hv, err := HGet("h", "f")
	require.NoError(t, err)
	require.Equal(t, "hvalue", hv)
}
