package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockRedis2 – minimal RedisStore mock specialized for dedupe tests.
type mockRedis2 struct {
	mu    sync.Mutex
	calls []setCall
}

func (m *mockRedis2) GetValue(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockRedis2) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	m.mu.Lock()
	m.calls = append(m.calls, setCall{key: key, val: v, ttl: ttl})
	m.mu.Unlock()
	return nil
}

func (m *mockRedis2) Calls() []setCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]setCall(nil), m.calls...)
}

// constant key factory
func constKey(k string) func(TaskFunc) string {
	return func(TaskFunc) string { return k }
}

// simple successful task
func trivialTask(v string) TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		return v, nil, nil, nil
	}
}

// TestDedupe_SameKeyReusesTask checks that two SubmitTask calls with same dedupe key reuse ID and channel.
func TestDedupe_SameKeyReusesTask(t *testing.T) {
	block := make(chan struct{}) // prevents task from finishing immediately

	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
		TaskKey:    func(f TaskFunc) string { return "dedupe" },
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	id1, ch1, err := wp.SubmitTask(func(ctx context.Context) (interface{}, []string, []string, []error) {
		<-block
		return "ok", nil, nil, nil
	}, Medium, 1)
	require.NoError(t, err)

	// Task is now inflight (blocked)

	id2, ch2, err := wp.SubmitTask(func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "ok", nil, nil, nil
	}, Medium, 1)
	require.NoError(t, err)

	require.Equal(t, id1, id2, "dedupe should return same task ID")
	require.Equal(t, ch1, ch2, "same dedupe key must return same channel")

	// release the block so worker can finish
	close(block)

	<-ch1 // consume result
}

// TestDedupe_SeparateKeysAreIndependent ensures that different keys create separate tasks.
func TestDedupe_SeparateKeysAreIndependent(t *testing.T) {
	opts := &RunOptions{
		TaskKey:    func(f TaskFunc) string { return "k1" }, // we'll override per call
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	rdb := &mockRedis2{}
	wp := NewWorkerPool(opts, 1, rdb)
	defer wp.Stop()

	// Override TaskKey dynamically
	opts.TaskKey = constKey("k1")
	id1, ch1, err := wp.SubmitTask(trivialTask("one"), Medium, 1)
	require.NoError(t, err)

	opts.TaskKey = constKey("k2")
	id2, ch2, err := wp.SubmitTask(trivialTask("two"), Medium, 1)
	require.NoError(t, err)

	require.NotEqual(t, id1, id2, "two different keys => two different tasks")
	require.NotEqual(t, ch1, ch2, "different dedupe keys => different channels")

	// consume both
	res1 := <-ch1
	res2 := <-ch2
	require.Equal(t, "one", res1.Result)
	require.Equal(t, "two", res2.Result)
}

// TestDedupe_InflightExistsButTaskNotFound exercises the fallback path in SubmitTask.
func TestDedupe_InflightExistsButTaskNotFound(t *testing.T) {
	opts := &RunOptions{
		TaskKey:    constKey("ghost-key"),
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	rdb := &mockRedis2{}
	wp := NewWorkerPool(opts, 1, rdb)

	// manually insert inflight entry but do NOT insert task in any worker queue
	wp.mu.Lock()
	wp.inflight["ghost-key"] = inflightEntry{id: 999} // bogus ID
	wp.mu.Unlock()

	// Now submit a real task; SubmitTask should detect missing queue entry and create new task.
	id, ch, err := wp.SubmitTask(trivialTask("ghost"), Medium, 1)
	require.NoError(t, err)
	require.NotEqual(t, int64(999), id, "fallback must create fresh ID")
	require.NotNil(t, ch)

	// clean up
	wp.Stop()

	// ensure Redis got at least one SetValue call
	require.Eventually(t, func() bool {
		return len(rdb.Calls()) > 0
	}, time.Second, 10*time.Millisecond, "expected Redis SetValue call")

}

// TestDedupe_RedisSetCalledOnSubmit ensures dedupe writes an entry into Redis.
func TestDedupe_RedisSetCalledOnSubmit(t *testing.T) {
	opts := &RunOptions{
		TaskKey:    constKey("redis-save"),
		MaxRetries: 0,
		Timeout:    time.Second,
	}

	rdb := &mockRedis2{}
	wp := NewWorkerPool(opts, 1, rdb)
	defer wp.Stop()

	_, ch, err := wp.SubmitTask(trivialTask("res"), Medium, 1)
	require.NoError(t, err)

	// consume result
	<-ch

	// give async goroutines a moment
	time.Sleep(50 * time.Millisecond)

	require.NotEmpty(t, rdb.Calls(), "Redis SetValue should have been called at least once")
}
