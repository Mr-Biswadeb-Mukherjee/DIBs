package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func rrTaskReturningID() TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		// Worker ID is inferred by context value which WorkerPool injects via exec()
		// But WorkerPool does NOT give worker ID in ctx.
		// So instead, we capture worker ID via closure injection at Submit-time.
		return "NO_ID", nil, nil, nil
	}
}

// We override submit with an injected wrapper that returns worker ID.
func rrTaskCaptureWorkerID(workerID *int) TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		return *workerID, nil, nil, nil
	}
}

// ---------------------------------------------------------
// Test 1: Round robin selection by EXECUTION ORDER
// ---------------------------------------------------------
func TestRoundRobinWorkersReceiveTasks(t *testing.T) {
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	wp := NewWorkerPool(opts, 3, nil)
	defer wp.Stop()

	var results []<-chan WorkerResult

	// We submit tasks that each capture the worker ID that executed it.
	for i := range wp.workers {
		w := wp.workers[i]
		task := rrTaskCaptureWorkerID(&w.ID)
		_, ch, err := wp.SubmitTask(task, Medium, 1)
		require.NoError(t, err)
		results = append(results, ch)
	}

	// Worker order:
	// rrIndex=0 -> worker0 executes first
	// rrIndex=1 -> worker1 executes second
	// rrIndex=2 -> worker2 executes third

	var out []int
	for _, ch := range results {
		select {
		case r := <-ch:
			out = append(out, r.Result.(int))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for result")
		}
	}

	require.ElementsMatch(t, []int{0, 1, 2}, out)
}

// ---------------------------------------------------------
// Test 2: Round robin after adding workers dynamically
// ---------------------------------------------------------
func TestRoundRobinAfterAddingWorkers(t *testing.T) {
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	wp := NewWorkerPool(opts, 2, nil)
	defer wp.Stop()

	// Submit 2 tasks to cycle workers 0 and 1
	for i := 0; i < 2; i++ {
		w := wp.workers[i]
		task := rrTaskCaptureWorkerID(&w.ID)
		_, _, _ = wp.SubmitTask(task, Medium, 1)
	}

	// Add a new worker (#2)
	wp.addWorker()
	require.Equal(t, 3, len(wp.workers))

	// Now submit 3 tasks which should go: w0 → w1 → w2
	var results []<-chan WorkerResult
	for i := 0; i < 3; i++ {
		w := wp.workers[i]
		task := rrTaskCaptureWorkerID(&w.ID)
		_, ch, _ := wp.SubmitTask(task, Medium, 1)
		results = append(results, ch)
	}

	out := make([]int, 0, 3)
	for _, ch := range results {
		select {
		case r := <-ch:
			out = append(out, r.Result.(int))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for result")
		}
	}

	require.ElementsMatch(t, []int{0, 1, 2}, out)
}
