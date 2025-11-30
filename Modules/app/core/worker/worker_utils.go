//worker_utils.go: Internal helpers for retries, logging, dedupe, and result handling.

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func taskKey(id int64) string {
	return fmt.Sprintf("task:%d", id)
}

func taskResultKey(id int64) string {
	return fmt.Sprintf("task:%d:result", id)
}

func taskStatusKey(id int64) string {
	return fmt.Sprintf("task:%d:status", id)
}

func taskDedupeKey(key string) string {
	return fmt.Sprintf("dedupe:%s", key)
}

func workerStateKey(id int) string {
	return fmt.Sprintf("worker:%d:state", id)
}

// Queue key for Redis ZSET
func redisQueueKey() string {
	return "tasks:queue"
}

func logMessage(ch chan string, msg string, nonblock bool) {
	if ch == nil {
		return
	}
	if nonblock {
		select {
		case ch <- msg:
		default:
		}
		return
	}
	ch <- msg
}

const (
	priorityWeight = 1e12
)

func computePriorityScore(p TaskPriority, t time.Time) float64 {
	base := float64(p+1) * priorityWeight
	return -base + float64(t.UnixNano())
}

func encodeResult(res WorkerResult) (string, error) {
	b, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeResult(s string) (WorkerResult, error) {
	var r WorkerResult
	err := json.Unmarshal([]byte(s), &r)
	return r, err
}

type retryConfig struct {
	MaxRetries int
	Timeout    time.Duration
}

func executeWithRetries(ctx context.Context, t *Task, cfg retryConfig) WorkerResult {
	timeout := cfg.Timeout
	maxR := cfg.MaxRetries

	var result interface{}
	var info, warn []string
	var errs []error

	for attempt := 0; attempt <= maxR; attempt++ {
		subCtx, cancel := context.WithTimeout(ctx, timeout)
		result, info, warn, errs = t.Func(subCtx)
		cancel()

		if len(errs) == 0 {
			break
		}
	}

	return WorkerResult{
		Result:   result,
		Info:     info,
		Warnings: warn,
		Errors:   errs,
	}
}

func computeDedupeKey(opts *RunOptions, f TaskFunc) string {
	if opts == nil || opts.TaskKey == nil {
		return ""
	}
	return opts.TaskKey(f)
}

func sendResult(ch chan WorkerResult, r WorkerResult) {
	select {
	case ch <- r:
	default:
	}
}

func clampLoad(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// Format log prefix : message
func logFormat(prefix string, msg string) string {
	return fmt.Sprintf("%s: %s", prefix, msg)
}

func readWithTimeout(ctx context.Context, fn func(context.Context) (string, error)) (string, error) {
	type out struct {
		val string
		err error
	}
	ch := make(chan out, 1)

	go func() {
		v, err := fn(ctx)
		ch <- out{v, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case o := <-ch:
		return o.val, o.err
	}
}
