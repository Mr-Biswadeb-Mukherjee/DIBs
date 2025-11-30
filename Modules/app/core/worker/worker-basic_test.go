package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockRedis is a tiny test double for RedisStore that records SetValue calls.
type mockRedis struct {
	mu    sync.Mutex
	calls []setCall
}

type setCall struct {
	key string
	val interface{}
	ttl time.Duration
}

func (m *mockRedis) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockRedis) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	m.mu.Lock()
	m.calls = append(m.calls, setCall{key: key, val: v, ttl: ttl})
	m.mu.Unlock()
	return nil
}

func (m *mockRedis) Calls() []setCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]setCall(nil), m.calls...)
}

// TestNewWorkerPoolDefaults checks that defaults are applied and workers are created.
func TestNewWorkerPoolDefaults(t *testing.T) {
	wp := NewWorkerPool(nil, 2, nil)
	defer wp.Stop()

	require.NotNil(t, wp)
	require.NotNil(t, wp.options)
	require.Equal(t, 2, len(wp.workers))
	require.Equal(t, DefaultTimeout, wp.options.Timeout)
	require.Equal(t, DefaultEvalInterval, wp.options.EvalInterval)
}

// TestNewWorkerPoolClamping ensures conc is clamped to MinWorkers when low.
func TestNewWorkerPoolClamping(t *testing.T) {
	opts := &RunOptions{
		MinWorkers: 3,
		MaxWorkers: 10,
	}
	wp := NewWorkerPool(opts, 1, nil) // conc < MinWorkers => should become MinWorkers
	defer wp.Stop()

	require.NotNil(t, wp)
	require.Equal(t, 3, len(wp.workers))
	require.True(t, wp.options.MinWorkers == 3)
}

// TestSubmitTaskNil verifies SubmitTask rejects a nil TaskFunc.
func TestSubmitTaskNil(t *testing.T) {
	wp := NewWorkerPool(nil, 1, nil)
	defer wp.Stop()

	_, _, err := wp.SubmitTask(nil, Medium, 1)
	require.Error(t, err)
}

// TestSubmitTaskRoundRobinAndResult submits two quick tasks and ensures results arrive.
func TestSubmitTaskRoundRobinAndResult(t *testing.T) {
	wp := NewWorkerPool(nil, 2, nil)
	defer wp.Stop()

	// simple task that returns immediately
	makeTask := func(v string) TaskFunc {
		return func(ctx context.Context) (interface{}, []string, []string, []error) {
			return v, nil, nil, nil
		}
	}

	id1, ch1, err := wp.SubmitTask(makeTask("one"), Low, 1)
	require.NoError(t, err)
	require.NotZero(t, id1)
	id2, ch2, err := wp.SubmitTask(makeTask("two"), Low, 1)
	require.NoError(t, err)
	require.NotZero(t, id2)

	// wait for both results
	select {
	case r := <-ch1:
		require.Equal(t, "one", r.Result)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for first task result")
	}

	select {
	case r := <-ch2:
		require.Equal(t, "two", r.Result)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second task result")
	}
}

// TestDedupeInflightLifecycle ensures inflight is set on SubmitTask and cleared after execution.
func TestDedupeInflightLifecycle(t *testing.T) {
	// TaskKey returns a constant dedupe key for testing.
	opts := &RunOptions{
		TaskKey: func(f TaskFunc) string {
			return "constant-key"
		},
		Timeout:    1 * time.Second,
		MaxRetries: 0,
	}

	mock := &mockRedis{}
	wp := NewWorkerPool(opts, 1, mock)
	defer wp.Stop()

	// task that returns immediately
	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "done", nil, nil, nil
	}

	// submit
	_, ch, err := wp.SubmitTask(task, Medium, 1)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// inflight should contain the key immediately
	wp.mu.Lock()
	_, ok := wp.inflight["constant-key"]
	wp.mu.Unlock()
	require.True(t, ok, "inflight should contain dedupe key after submit")

	// wait for result
	select {
	case r := <-ch:
		require.Equal(t, "done", r.Result)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task result")
	}

	// give exec's deferred goroutine a moment to clear inflight and call redis
	time.Sleep(50 * time.Millisecond)

	wp.mu.Lock()
	_, ok = wp.inflight["constant-key"]
	wp.mu.Unlock()
	require.False(t, ok, "inflight should be cleared after task execution")

	// ensure Redis got at least one SetValue call (initial set and later TTL shrink)
	calls := mock.Calls()
	require.GreaterOrEqual(t, len(calls), 1)
}
