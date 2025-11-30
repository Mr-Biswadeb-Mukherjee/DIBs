package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------
// TEST monitor() exit conditions
// ------------------------------------------------------------

func TestMonitor_StopsOnContextCancel(t *testing.T) {
	opts := &RunOptions{
		AutoScale:    true,
		EvalInterval: 10 * time.Millisecond,
		MinWorkers:   1,
		MaxWorkers:   3,
	}

	tick := make(chan time.Time)
	wp := NewWorkerPoolWithTick(opts, 1, nil, tick)

	// Cancel context by Stop()
	wp.Stop()

	// monitor goroutine should exit immediately (monitorStop closed)
	select {
	case <-wp.monitorStop:
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("monitor did not stop on context cancel")
	}
}

// ------------------------------------------------------------
// TEST removeIdle behavior
// ------------------------------------------------------------

func TestRemoveIdle_NoIdleWorkers(t *testing.T) {
	opts := &RunOptions{
		AutoScale:    true,
		MinWorkers:   1,
		MaxWorkers:   3,
		EvalInterval: 20 * time.Millisecond,
	}

	wp := NewWorkerPool(opts, 2, nil)
	defer wp.Stop()

	// Make both workers look busy — no scale down possible
	for _, w := range wp.workers {
		w.mu.Lock()
		w.Load = 2
		w.mu.Unlock()
	}

	// removeIdle should do nothing
	wp.removeIdle()

	require.Equal(t, 2, len(wp.workers))
}

func TestRemoveIdle_RemovesExactlyOne(t *testing.T) {
	opts := &RunOptions{
		AutoScale:       true,
		MinWorkers:      1,
		MaxWorkers:      5,
		EvalInterval:    20 * time.Millisecond,
		IdleGracePeriod: time.Millisecond,
	}

	wp := NewWorkerPool(opts, 3, nil)
	defer wp.Stop()

	// Make last worker idle
	wp.workers[2].mu.Lock()
	wp.workers[2].Load = 0
	wp.workers[2].mu.Unlock()

	wp.removeIdle()

	require.Equal(t, 2, len(wp.workers))
}

// ------------------------------------------------------------
// TEST scaleEval edge cases
// ------------------------------------------------------------

func TestScaleEval_NoScaleAtMaxWorkers(t *testing.T) {
	opts := &RunOptions{
		AutoScale:        true,
		MinWorkers:       1,
		MaxWorkers:       2,
		ScaleUpThreshold: 0.1,
	}

	wp := NewWorkerPool(opts, 2, nil)
	defer wp.Stop()

	// generate backlog but already max workers
	for i := 0; i < 10; i++ {
		_, _, _ = wp.SubmitTask(makeTask("b"), Medium, 1)
	}

	wp.scaleEval()

	require.Equal(t, 2, len(wp.workers), "must not exceed MaxWorkers")
}

func makeTask(v interface{}) TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		return v, nil, nil, nil
	}
}

func TestScaleEval_LowLoadResetLastLowLoadAt(t *testing.T) {
	opts := &RunOptions{
		AutoScale:          true,
		MinWorkers:         1,
		MaxWorkers:         3,
		ScaleDownThreshold: 0.1,
		IdleGracePeriod:    10 * time.Millisecond,
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	// Simulate load spike
	w := wp.workers[0]
	w.mu.Lock()
	w.Load = 5
	w.mu.Unlock()

	// First scaleEval should reset lastLowLoadAt to zero
	wp.scaleEval()
	require.True(t, wp.lastLowLoadAt.IsZero(), "lastLowLoadAt should reset on high load")
}

func TestScaleEval_EqualThreshold_NoScale(t *testing.T) {
	opts := &RunOptions{
		AutoScale:          true,
		MinWorkers:         1,
		MaxWorkers:         5,
		ScaleUpThreshold:   1.0,
		ScaleDownThreshold: 1.0,
	}

	wp := NewWorkerPool(opts, 1, nil)
	defer wp.Stop()

	// Simulate exactly equal conditions
	w := wp.workers[0]
	w.mu.Lock()
	w.Load = 1
	w.taskQueue.Push(&Task{
		ID:       99,
		Priority: Medium,
		Created:  time.Now(),
		Func: func(ctx context.Context) (interface{}, []string, []string, []error) {
			return nil, nil, nil, nil
		},
	})

	w.mu.Unlock()

	wp.scaleEval()

	// No scale up/down expected at equality
	require.Equal(t, 1, len(wp.workers))
}
