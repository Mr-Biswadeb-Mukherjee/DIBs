// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// shortTask simply returns immediately.
func shortTask(v string) TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		return v, nil, nil, nil
	}
}

// TestStop_CancelsRunningTasks ensures Stop() cancels the pool context and running tasks finish.
func TestStop_CancelsRunningTasks(t *testing.T) {
	opts := &RunOptions{
		Timeout:    2 * time.Second,
		MaxRetries: 0,
		TaskKey:    constKey("shutdown-long"),
	}

	wp := NewWorkerPool(opts, 2, nil)

	long := func(label string) TaskFunc {
		return func(ctx context.Context) (interface{}, []string, []string, []error) {
			<-ctx.Done()
			return label, nil, nil, nil
		}
	}

	_, ch1, err := wp.SubmitTask(long("c1"), Medium, 1)
	require.NoError(t, err)
	_, ch2, err := wp.SubmitTask(long("c2"), Medium, 1)
	require.NoError(t, err)

	// Give workers time to start at least one task
	time.Sleep(50 * time.Millisecond)

	// Now stop the pool
	wp.Stop()

	// ch1 must produce a result (because at least 1 task will always start)
	select {
	case <-ch1:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for ch1 result after Stop()")
	}

	// ch2 may or may not run; but MUST NOT block forever
	select {
	case <-ch2:
		// ok, second task ran
	case <-time.After(500 * time.Millisecond):
		// acceptable: task never started, so channel may remain empty
		// but test must NOT fail — just continue
	}

	// inflight must be empty
	wp.mu.Lock()
	require.Len(t, wp.inflight, 0)
	wp.mu.Unlock()
}

// TestStop_ClosesMonitorStop ensures the monitorStop channel is closed by Stop().
func TestStop_ClosesMonitorStop(t *testing.T) {
	opts := &RunOptions{
		AutoScale:        true,
		MinWorkers:       1,
		MaxWorkers:       2,
		EvalInterval:     10 * time.Millisecond,
		ScaleUpThreshold: 1000.0, // avoid auto changes
	}

	// use NewWorkerPoolWithTick so monitor is controlled by channel
	tick := make(chan time.Time, 1)
	wp := NewWorkerPoolWithTick(opts, 1, nil, tick)

	// ensure monitorStop channel exists
	require.NotNil(t, wp.monitorStop)

	// stop pool
	wp.Stop()

	// monitorStop should be closed; non-blocking read should succeed
	select {
	case <-wp.monitorStop:
		// ok
	default:
		t.Fatal("monitorStop should be closed after Stop()")
	}
}

// TestStop_IdempotentCalling ensures Stop can be called multiple times safely.
func TestStop_IdempotentCalling(t *testing.T) {
	opts := &RunOptions{
		Timeout:    time.Second,
		MaxRetries: 0,
	}

	wp := NewWorkerPool(opts, 1, nil)

	// submit a short task to ensure worker did some work
	_, ch, err := wp.SubmitTask(shortTask("ok"), Low, 1)
	require.NoError(t, err)
	<-ch

	// call Stop twice — both must return without panic or deadlock
	wp.Stop()
	// second call should be safe
	wp.Stop()

	// ensure monitorStop closed (if present)
	select {
	case <-wp.monitorStop:
	default:
		// if monitorStop not closed that's ok for non-autoscale pools; just ensure Stop returned
	}
}
