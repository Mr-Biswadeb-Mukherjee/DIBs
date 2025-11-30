package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Redis mock for exec tests
type execRedisMock struct {
	mu    sync.Mutex
	set   []setCall
	getKV map[string]string
}

func (m *execRedisMock) GetValue(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getKV[key], nil
}

func (m *execRedisMock) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	m.mu.Lock()
	m.set = append(m.set, setCall{key: key, val: v, ttl: ttl})
	m.mu.Unlock()
	return nil
}

func (m *execRedisMock) Calls() []setCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]setCall(nil), m.set...)
}

// ------------------------------------------------------------
// Execution Tests
// ------------------------------------------------------------

func TestExec_BasicExecutionAndLoad(t *testing.T) {
	logCh := make(chan string, 10)
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
		LogChannel: logCh,
		TaskKey:    constKey("exec-basic"),
	}

	rdb := &execRedisMock{}
	wp := NewWorkerPool(opts, 1, rdb)
	defer wp.Stop()

	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "done", []string{"hello"}, []string{"warn"}, nil
	}

	id, ch, err := wp.SubmitTask(task, Medium, 1)
	require.NoError(t, err)
	require.NotZero(t, id)

	// Wait until worker picked up the task:
	// queue length must drop to 0.
	w := wp.workers[0]
	require.Eventually(t, func() bool {
		w.mu.Lock()
		n := w.taskQueue.Len()
		w.mu.Unlock()
		return n == 0
	}, time.Second, 5*time.Millisecond, "worker never picked up task")

	// Collect result
	var res WorkerResult
	select {
	case res = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for result")
	}

	require.Equal(t, "done", res.Result)

	// load eventually goes back to 0
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.Load == 0
	}, time.Second, 5*time.Millisecond)

	// inflight cleared
	require.Eventually(t, func() bool {
		wp.mu.Lock()
		_, exists := wp.inflight["exec-basic"]
		wp.mu.Unlock()
		return !exists
	}, time.Second, 5*time.Millisecond)

	// collect logs
	close(logCh)
	hasLogs := false
	for range logCh {
		hasLogs = true
	}
	require.True(t, hasLogs, "expected logs during exec")
}

// ------------------------------------------------------------

func TestExec_RetriesAndErrors(t *testing.T) {
	attempts := 0
	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		attempts++
		if attempts < 3 {
			return nil, nil, nil, []error{context.DeadlineExceeded}
		}
		return "ok", nil, nil, nil
	}

	opts := &RunOptions{
		Timeout:    30 * time.Millisecond,
		MaxRetries: 3,
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	_, ch, err := wp.SubmitTask(task, High, 1)
	require.NoError(t, err)

	var r WorkerResult
	select {
	case r = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for retry result")
	}

	require.Equal(t, "ok", r.Result)
	require.GreaterOrEqual(t, attempts, 3)
}

// ------------------------------------------------------------

func TestExec_Callbacks(t *testing.T) {
	var startID, finishID int64
	var finishResult WorkerResult
	lock := sync.Mutex{}

	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
		TaskKey:    constKey("cb-key"),
		OnTaskStart: func(id int64) {
			lock.Lock()
			startID = id
			lock.Unlock()
		},
		OnTaskFinish: func(id int64, r WorkerResult) {
			lock.Lock()
			finishID = id
			finishResult = r
			lock.Unlock()
		},
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "X", nil, nil, nil
	}

	id, ch, err := wp.SubmitTask(task, High, 1)
	require.NoError(t, err)

	<-ch // wait for completion

	lock.Lock()
	require.Equal(t, id, startID)
	require.Equal(t, id, finishID)
	require.Equal(t, "X", finishResult.Result)
	lock.Unlock()
}

// ------------------------------------------------------------

func TestExec_RedisTTL_Shrink(t *testing.T) {
	opts := &RunOptions{
		TaskKey:    constKey("redis-ttl"),
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	rdb := &execRedisMock{}
	wp := NewWorkerPool(opts, 1, rdb)
	defer wp.Stop()

	task := func(ctx context.Context) (interface{}, []string, []string, []error) {
		return "Z", nil, nil, nil
	}

	_, ch, err := wp.SubmitTask(task, Medium, 1)
	require.NoError(t, err)

	<-ch // consume result

	// wait for TTL shrink goroutine
	time.Sleep(50 * time.Millisecond)

	calls := rdb.Calls()
	require.NotEmpty(t, calls, "expected Redis TTL shrink call")

	foundTTL1s := false
	for _, c := range calls {
		if c.ttl <= time.Second {
			foundTTL1s = true
		}
	}
	require.True(t, foundTTL1s, "should eventually set TTL around 1 second")
}
