package worker

import (
	"context"
	"fmt"
	"time"
)

func (wp *WorkerPool) exec(w *workerInfo, t *Task) {
	// update load
	w.mu.Lock()
	w.Load += t.Weight
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.Load -= t.Weight
		if w.Load < 0 {
			w.Load = 0
		}
		w.mu.Unlock()

		// clear dedupe entry
		if t.Dedupe != "" {
			wp.mu.Lock()
			// only delete if the stored id matches (defensive)
			if e, ok := wp.inflight[t.Dedupe]; ok && e.id == t.ID {
				delete(wp.inflight, t.Dedupe)
			}
			wp.mu.Unlock()

			// Best-effort Redis cleanup: shrink TTL instead of DEL.
			if wp.redis != nil {
				dedupeKey := fmt.Sprintf("inflight:%s", t.Dedupe)
				go func(dk string) {
					ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
					// set a tiny TTL (1 second) so Redis expires it almost immediately
					_ = wp.redis.SetValue(ctx, dk, "", 1*time.Second)
					cancel()
				}(dedupeKey)
			}
		}
	}()

	// callback: start
	if wp.options.OnTaskStart != nil {
		wp.options.OnTaskStart(t.ID)
	}

	// run with retries + timeout (executeWithRetries is in worker_utils.go)
	res := executeWithRetries(wp.ctx, t, retryConfig{
		MaxRetries: wp.options.MaxRetries,
		Timeout:    wp.options.Timeout,
	})

	// log info/warn/errors
	for _, m := range res.Info {
		wp.log(fmt.Sprintf("INFO task %d: %s", t.ID, m))
	}
	for _, m := range res.Warnings {
		wp.log(fmt.Sprintf("WARN task %d: %s", t.ID, m))
	}
	for _, e := range res.Errors {
		if e != nil {
			wp.log(fmt.Sprintf("ERROR task %d: %v", t.ID, e))
		}
	}

	// deliver result non-blocking
	sendResult(t.ResultCh, res)

	// callback: finish
	if wp.options.OnTaskFinish != nil {
		wp.options.OnTaskFinish(t.ID, res)
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
		wp.mu.Lock()
		if len(wp.workers) < wp.options.MaxWorkers {
			wp.addWorker()
			wp.log(fmt.Sprintf("Autoscale: up to %d", len(wp.workers)))
		}
		wp.mu.Unlock()
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
