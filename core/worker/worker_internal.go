// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package worker

import (
	"context"
	"fmt"
	"time"
)

func (wp *WorkerPool) exec(w *workerInfo, t *Task) {
	updateWorkerLoad(w, t.Weight)
	defer wp.cleanupTaskState(w, t)

	if wp.options.OnTaskStart != nil {
		wp.options.OnTaskStart(t.ID)
	}

	res := executeWithRetries(wp.ctx, t, retryConfig{
		MaxRetries: wp.options.MaxRetries,
		Timeout:    wp.options.Timeout,
	})
	wp.logTaskResult(t.ID, res)
	sendResult(t.ResultCh, res)
	if wp.options.OnTaskFinish != nil {
		wp.options.OnTaskFinish(t.ID, res)
	}
}

func updateWorkerLoad(w *workerInfo, delta int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Load += delta
	if w.Load < 0 {
		w.Load = 0
	}
}

func (wp *WorkerPool) cleanupTaskState(w *workerInfo, t *Task) {
	updateWorkerLoad(w, -t.Weight)
	if t.Dedupe == "" {
		return
	}
	wp.clearInflightDedupe(t.Dedupe, t.ID)
	wp.expireRedisDedupe(t.Dedupe)
}

func (wp *WorkerPool) clearInflightDedupe(key string, taskID int64) {
	wp.mu.Lock()
	if e, ok := wp.inflight[key]; ok && e.id == taskID {
		delete(wp.inflight, key)
	}
	wp.mu.Unlock()
}

func (wp *WorkerPool) expireRedisDedupe(key string) {
	if wp.redis == nil {
		return
	}
	dedupeKey := fmt.Sprintf("inflight:%s", key)
	go func(dk string) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		_ = wp.redis.SetValue(ctx, dk, "", time.Second)
		cancel()
	}(dedupeKey)
}

func (wp *WorkerPool) logTaskResult(taskID int64, res WorkerResult) {
	for _, m := range res.Info {
		wp.log(fmt.Sprintf("INFO task %d: %s", taskID, m))
	}
	for _, m := range res.Warnings {
		wp.log(fmt.Sprintf("WARN task %d: %s", taskID, m))
	}
	for _, e := range res.Errors {
		if e != nil {
			wp.log(fmt.Sprintf("ERROR task %d: %v", taskID, e))
		}
	}
}

func applyDurationDefault(target *time.Duration, fallback time.Duration) {
	if *target <= 0 {
		*target = fallback
	}
}

func applyFloatDefault(target *float64, fallback float64) {
	if *target <= 0 {
		*target = fallback
	}
}

func applyIntMin(target *int, min int) {
	if *target < min {
		*target = min
	}
}

func (wp *WorkerPool) monitor(tickCh <-chan time.Time) {
	for {
		select {
		case <-tickCh:
			wp.scaleEval()
		case <-wp.monitorStop:
			return
		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) scaleEval() {
	wp.mu.Lock()
	ws := append([]*workerInfo{}, wp.workers...)
	wp.mu.Unlock()

	if len(ws) == 0 {
		return
	}

	totalLoad := 0
	totalQueue := 0

	for _, w := range ws {
		w.mu.Lock()
		totalLoad += w.Load
		totalQueue += w.taskQueue.Len()
		w.mu.Unlock()
	}

	n := float64(len(ws))
	avg := float64(totalLoad) / n
	back := float64(totalQueue) / n

	// scale up if backlog per worker exceeds threshold
	if back > wp.options.ScaleUpThreshold {
		scaled := false
		wp.mu.Lock()
		if len(wp.workers) < wp.options.MaxWorkers {
			scaled = true
		}
		wp.mu.Unlock()

		if scaled {
			wp.addWorker()
			wp.mu.Lock()
			count := len(wp.workers)
			wp.mu.Unlock()
			wp.log(fmt.Sprintf("Autoscale: up to %d", count))
		}
		wp.lastLowLoadAt = time.Time{}
		return
	}

	// scale down if idle for grace period
	if avg < wp.options.ScaleDownThreshold && back < wp.options.ScaleDownThreshold {
		if wp.lastLowLoadAt.IsZero() {
			wp.lastLowLoadAt = time.Now()
			return
		}
		if time.Since(wp.lastLowLoadAt) >= wp.options.IdleGracePeriod {
			wp.removeIdle()
		}
	} else {
		wp.lastLowLoadAt = time.Time{}
	}
}

func (wp *WorkerPool) removeIdle() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.workers) <= wp.options.MinWorkers {
		return
	}

	for i := len(wp.workers) - 1; i >= 0; i-- {
		w := wp.workers[i]
		w.mu.Lock()
		idle := w.Load == 0 && w.taskQueue.Len() == 0 && !w.closing
		if idle {
			w.closing = true
			close(w.stop)
			w.cond.Broadcast()
			// remove slice element
			wp.workers = append(wp.workers[:i], wp.workers[i+1:]...)
			w.mu.Unlock()
			wp.log(fmt.Sprintf("Autoscale: down to %d", len(wp.workers)))
			return
		}
		w.mu.Unlock()
	}
}

func stopWorkerPoolInternal(wp *WorkerPool) {
	// stop monitor
	select {
	case <-wp.monitorStop:
		// already closed or stopped; continue
	default:
		close(wp.monitorStop)
	}
	// cancel running tasks
	wp.cancel()

	wp.mu.Lock()
	for _, w := range wp.workers {
		w.mu.Lock()
		if !w.closing {
			w.closing = true
			close(w.stop)
			w.cond.Broadcast()
		}
		w.mu.Unlock()
	}

	// clear inflight registry to avoid stale local entries
	wp.inflight = make(map[string]inflightEntry)

	wp.mu.Unlock()

	// wait until worker goroutines complete
	wp.wg.Wait()
}
