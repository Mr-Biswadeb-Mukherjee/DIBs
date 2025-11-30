package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Fake Redis for reduced TTL dedupe cleanup testing
type fakeRedis struct {
	mu      sync.Mutex
	setKeys []string
}

func (r *fakeRedis) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (r *fakeRedis) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setKeys = append(r.setKeys, key)
	return nil
}

// Simple task which returns logs + warnings + errors
func execTask() TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "RESULT", []string{"i1"}, []string{"w1"}, []error{errors.New("e1")}
	}
}

func TestExec_CallbacksAndLogging(t *testing.T) {
	logCh := make(chan string, 10)

	startCalls := 0
	finishCalls := 0

	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
		LogChannel: logCh,
		OnTaskStart: func(taskID int64) {
			startCalls++
		},
		OnTaskFinish: func(taskID int64, res WorkerResult) {
			finishCalls++
		},
		NonBlockingLogs: false,
		TaskKey: func(f TaskFunc) string {
			return "dedupe-cb"
		},
	}

	redis := &fakeRedis{}
	wp := NewWorkerPool(opts, 1, redis)
	defer wp.Stop()

	_, ch, err := wp.SubmitTask(execTask(), Medium, 1)
	require.NoError(t, err)

	var res WorkerResult
	select {
	case res = <-ch:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}

	// Result returned correctly
	require.Equal(t, "RESULT", res.Result)
	require.Equal(t, []string{"i1"}, res.Info)
	require.Equal(t, []string{"w1"}, res.Warnings)
	require.Len(t, res.Errors, 1)

	// Callbacks executed
	require.Equal(t, 1, startCalls)
	require.Equal(t, 1, finishCalls)

	// Logs captured
	close(logCh)
	var count int
	for range logCh {
		count++
	}
	require.Greater(t, count, 0, "expected logs for info/warn/errors")

	// Dedupe cleanup via Redis should have been triggered
	redis.mu.Lock()
	found := false
	for _, k := range redis.setKeys {
		if k == "inflight:dedupe-cb" {
			found = true
		}
	}
	redis.mu.Unlock()
	require.True(t, found, "Redis dedupe cleanup should be called")
}

// ------------------------------------------------------------
// Test branch where Dedupe = "" (no dedupe cleanup should run)
// ------------------------------------------------------------
func TestExec_NoDedupeBranch(t *testing.T) {
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
		LogChannel: make(chan string, 5),
		// no TaskKey provided → Dedupe becomes ""
	}

	redis := &fakeRedis{}
	wp := NewWorkerPool(opts, 1, redis)
	defer wp.Stop()

	_, ch, err := wp.SubmitTask(execTask(), Medium, 1)
	require.NoError(t, err)

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	redis.mu.Lock()
	require.Equal(t, 0, len(redis.setKeys), "Redis should not be used when dedupe key empty")
	redis.mu.Unlock()
}

// ------------------------------------------------------------
// Test load increase and load clamp going back to 0
// ------------------------------------------------------------
func TestExec_LoadIncreaseAndClamp(t *testing.T) {
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	// Task that yields immediately
	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return nil, nil, nil, nil
	}

	// Submit with weight
	_, ch, err := wp.SubmitTask(task, Medium, 5)
	require.NoError(t, err)

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Load must have returned to zero after execution
	w := wp.workers[0]
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.Load == 0
	}, 500*time.Millisecond, 5*time.Millisecond)
}
