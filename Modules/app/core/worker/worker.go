package worker

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------
// UNCHANGEABLE API FUNCTION
// ---------------------------------------------------------

// Note: third parameter name changed from '_' -> rdb so we can store the
// injected RedisStore in the WorkerPool. Type/signature otherwise unchanged.
func NewWorkerPool(opts *RunOptions, conc int, rdb RedisStore) *WorkerPool {
	if opts == nil {
		opts = &RunOptions{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.EvalInterval <= 0 {
		opts.EvalInterval = DefaultEvalInterval
	}
	if opts.ScaleUpThreshold <= 0 {
		opts.ScaleUpThreshold = DefaultScaleUp
	}
	if opts.ScaleDownThreshold <= 0 {
		opts.ScaleDownThreshold = DefaultScaleDown
	}
	if opts.IdleGracePeriod <= 0 {
		opts.IdleGracePeriod = DefaultIdleGrace
	}
	if opts.MinWorkers < 1 {
		opts.MinWorkers = 1
	}

	// dedupe TTL default: twice the timeout if not set
	if opts.DedupeTTL <= 0 {
		opts.DedupeTTL = opts.Timeout * 2
	}

	if conc <= 0 {
		conc = runtime.NumCPU()
	}

	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = conc * 2
	}
	if conc < opts.MinWorkers {
		conc = opts.MinWorkers
	}
	if conc > opts.MaxWorkers {
		conc = opts.MaxWorkers
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		options:     opts,
		ctx:         ctx,
		cancel:      cancel,
		inflight:    make(map[string]inflightEntry),
		monitorStop: make(chan struct{}),
		redis:       rdb, // store injected Redis instance (may be nil)
	}

	// initialize workers
	for i := 0; i < conc; i++ {
		wp.addWorker()
	}

	if opts.AutoScale {
		tick := time.NewTicker(opts.EvalInterval)
		go wp.monitor(tick.C)
	}

	return wp
}

func (wp *WorkerPool) SubmitTask(f TaskFunc, p TaskPriority, weight int) (int64, <-chan WorkerResult, error) {
	if f == nil {
		return 0, nil, errors.New("nil task")
	}

	// compute dedupe key
	key := computeDedupeKey(wp.options, f)

	// lock and purge expired inflight entries, and check registry
	now := time.Now()
	wp.mu.Lock()
	if key != "" {
		for k, entry := range wp.inflight {
			if now.Sub(entry.created) > wp.options.DedupeTTL {
				delete(wp.inflight, k)
			}
		}
		if e, ok := wp.inflight[key]; ok {
			// return existing ID and channel (authoritative)
			ch := e.ch
			id := e.id
			wp.mu.Unlock()
			return id, ch, nil
		}
	}

	// if no workers currently, fail fast
	if len(wp.workers) == 0 {
		wp.mu.Unlock()
		return 0, nil, errors.New("no workers")
	}

	// create new task
	id := atomic.AddInt64(&wp.nextID, 1)
	ch := make(chan WorkerResult, 1)
	task := &Task{
		ID:       id,
		Func:     f,
		Priority: p,
		ResultCh: ch,
		Weight:   weight,
		Created:  time.Now(),
		Dedupe:   key,
	}

	// register inflight entry (authoritative local registry)
	if key != "" {
		wp.inflight[key] = inflightEntry{
			id:      id,
			ch:      ch,
			created: time.Now(),
		}

		// Best-effort: mark inflight in Redis so other processes can see the dedupe.
		// We don't block on Redis — it's a helper for distributed dedupe only.
		if wp.redis != nil {
			dedupeKey := fmt.Sprintf("inflight:%s", key)
			go func(dk string, val int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				// store id as string; set a reasonable TTL so abandoned entries expire.
				_ = wp.redis.SetValue(ctx, dk, fmt.Sprintf("%d", val), wp.options.DedupeTTL)
				cancel()
			}(dedupeKey, id)
		}
	}

	// round-robin distribution
	if len(wp.workers) == 0 {
		// shouldn't happen, but guard
		wp.mu.Unlock()
		return 0, nil, errors.New("no available workers")
	}
	wp.rrIndex = (wp.rrIndex + 1) % len(wp.workers)
	w := wp.workers[wp.rrIndex]
	wp.mu.Unlock()

	w.mu.Lock()
	heap.Push(&w.taskQueue, task)
	w.cond.Signal()
	w.mu.Unlock()

	return id, ch, nil
}

// Stop gracefully shuts down the pool.
func (wp *WorkerPool) Stop() {
	// delegate to internal shutdown helper
	stopWorkerPoolInternal(wp)
}

func (wp *WorkerPool) addWorker() {
	w := &workerInfo{
		ID:   len(wp.workers),
		stop: make(chan struct{}),
		wg:   &wp.wg,
	}
	w.cond = sync.NewCond(&w.mu)
	heap.Init(&w.taskQueue)

	wp.mu.Lock()
	wp.workers = append(wp.workers, w)
	wp.mu.Unlock()

	wp.wg.Add(1)
	go w.run(wp)
}

func (w *workerInfo) run(wp *WorkerPool) {
	defer w.wg.Done()
	for {
		w.mu.Lock()
		for w.taskQueue.Len() == 0 && !w.closing {
			select {
			case <-wp.ctx.Done():
				w.mu.Unlock()
				return
			case <-w.stop:
				w.mu.Unlock()
				return
			default:
			}
			w.cond.Wait()
		}
		if w.closing {
			w.mu.Unlock()
			return
		}
		item := heap.Pop(&w.taskQueue)
		w.mu.Unlock()

		if item == nil {
			continue
		}
		if t, ok := item.(*Task); ok {
			// wp.exec is implemented in worker_internal.go
			wp.exec(w, t)
		}
	}
}

func (wp *WorkerPool) log(msg string) {
	if wp.options == nil || wp.options.LogChannel == nil {
		return
	}
	logMessage(wp.options.LogChannel, msg, wp.options.NonBlockingLogs)
}

// NewWorkerPoolWithTick is used only for testing autoscaling.
// It injects a fake ticker channel so tests can trigger scaleEval()
// instantly without waiting for real time.
func NewWorkerPoolWithTick(opts *RunOptions, conc int, rdb RedisStore, tickCh <-chan time.Time) *WorkerPool {
	wp := NewWorkerPool(opts, conc, rdb)

	if opts.AutoScale && tickCh != nil {
		go wp.monitor(tickCh)
	}

	return wp
}
