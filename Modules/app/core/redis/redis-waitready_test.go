package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

//
// ===================================================================
//                 Minimal mock using embedding
// ===================================================================
//

type mockRedisClient struct {
	*redis.Client // implements UniversalClient
	pingErrs      []error
	call          int
}

func newMockClient() *mockRedisClient {
	fake := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:0", // unreachable, but unused
	})
	return &mockRedisClient{Client: fake}
}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	var err error
	if m.call < len(m.pingErrs) {
		err = m.pingErrs[m.call]
	}
	m.call++

	cmd := redis.NewStatusCmd(ctx)
	if err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

//
// ===================================================================
//                     Tests for waitForReady()
// ===================================================================
//

func TestWaitForReady_SucceedsAfterFailures(t *testing.T) {
	mock := newMockClient()
	mock.pingErrs = []error{
		errors.New("fail1"),
		errors.New("fail2"),
		nil,
	}

	r := &RedisClient{
		client: mock,
		cfg: &RedisConfig{
			BackoffMax: 1, // capped at 1 second
		},
		logger: silentLogger{},
	}

	start := time.Now()
	err := r.waitForReady(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, mock.call)

	elapsed := time.Since(start)

	// real expected = 1s sleep + 1s sleep ≈ 2 seconds
	require.Less(t, elapsed, 3500*time.Millisecond)
}

func TestWaitForReady_ContextTimeout(t *testing.T) {
	mock := newMockClient()
	mock.pingErrs = []error{
		errors.New("x"),
		errors.New("y"),
		errors.New("z"),
	}

	r := &RedisClient{
		client: mock,
		cfg: &RedisConfig{
			BackoffMax: 1,
		},
		logger: silentLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	err := r.waitForReady(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context deadline")
	require.GreaterOrEqual(t, mock.call, 1)
}

//
// ===================================================================
//            Tests for health checker (real timing)
// ===================================================================
//

func TestHealthChecker_Recovers(t *testing.T) {
	mock := newMockClient()
	mock.pingErrs = []error{
		errors.New("unhealthy"),
		nil,
	}

	r := &RedisClient{
		client: mock,
		cfg: &RedisConfig{
			HealthTick: 1, // ticker fires every 1 second
			BackoffMax: 1,
		},
		logger:   silentLogger{},
		stopChan: make(chan struct{}),
	}

	r.startHealthChecker()

	// Must wait >1 second so ticker fires
	time.Sleep(1200 * time.Millisecond)

	close(r.stopChan)
	r.wg.Wait()

	require.GreaterOrEqual(t, mock.call, 2)
}
