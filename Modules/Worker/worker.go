package Worker

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	Resources "github.com/official-biswadeb941/Infermal_v2/Modules/Resource"
)

type TaskFunc func(ctx context.Context) (interface{}, []string, []string, []error)

type WorkerResult struct {
	Result   interface{}
	Info     []string
	Warnings []string
	Errors   []error
}

type RunOptions struct {
	Timeout         time.Duration
	MaxRetries      int
	LogChannel      chan string
	OnTaskStart     func(taskID int64)
	OnTaskFinish    func(taskID int64, res WorkerResult)
	NonBlockingLogs bool

	AutoScale          bool
	MinWorkers         int
	MaxWorkers         int
	ScaleUpThreshold   float64
	ScaleDownThreshold float64
	IdleGracePeriod    time.Duration
	EvalInterval       time.Duration
}

type TaskPriority int

const (
	Low TaskPriority = iota
	Medium
	High
)

type Task struct {
	ID       int64
	Func     TaskFunc
	Priority TaskPriority
	ResultCh chan WorkerResult
	index    int
	Weight   int
	created  time.Time
}

type PriorityQueue []*Task

func (pq PriorityQueue) Len() int { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].created.Before(pq[j].created)
	}
	return pq[i].Priority > pq[j].Priority
}
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*Task)
	item.index = len(*pq)
	*pq = append(*pq, item)
}
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

type workerInfo struct {
	id        int
	load      int
	taskQueue PriorityQueue
	mu        sync.Mutex
	cond      *sync.Cond
	stop      chan struct{}
	wg        *sync.WaitGroup
	closing   bool
}

type WorkerPool struct {
	mu            sync.Mutex
	workers       []*workerInfo
	options       *RunOptions
	nextTaskID    int64
	concurrency   int
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	lastLowLoadAt time.Time
	monitorStop   chan struct{}
	rrIndex       int

	redis RedisStore // optional injected Redis store
}

// Minimal Redis interface used by worker pool. Declared here to avoid importing Redis module.
type RedisStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
}

const (
	defaultTimeout         = 10 * time.Second
	defaultScaleUpThresh   = 1.2
	defaultScaleDownThresh = 0.3
	defaultIdleGrace       = 10 * time.Second
	defaultEvalInterval    = 3 * time.Second
)

//
// NewWorkerPool now accepts an optional RedisStore parameter (pass nil if unused).
//
func NewWorkerPool(opts *RunOptions, concurrency int, redis RedisStore) *WorkerPool {
	if opts == nil {
		opts = &RunOptions{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.EvalInterval <= 0 {
		opts.EvalInterval = defaultEvalInterval
	}
	if opts.ScaleUpThreshold <= 0 {
		opts.ScaleUpThreshold = defaultScaleUpThresh
	}
	if opts.ScaleDownThreshold <= 0 {
		opts.ScaleDownThreshold = defaultScaleDownThresh
	}
	if opts.IdleGracePeriod <= 0 {
		opts.IdleGracePeriod = defaultIdleGrace
	}
	if opts.MinWorkers < 1 {
		opts.MinWorkers = 1
	}

	bestDevice, info, warn, errs := Resources.GetBestDevice()

	if concurrency <= 0 {
		switch bestDevice.Type {
		case Resources.CPU:
			if bestDevice.Cores > 0 {
				concurrency = bestDevice.Cores
			} else {
				concurrency = runtime.NumCPU()
			}
		case Resources.GPU:
			concurrency = 4
		case Resources.TPU:
			concurrency = 2
		default:
			concurrency = runtime.NumCPU()
		}
	}

	if opts.MaxWorkers <= 0 {
		switch bestDevice.Type {
		case Resources.CPU:
			if bestDevice.Cores > 0 {
				opts.MaxWorkers = bestDevice.Cores * 2
			} else {
				opts.MaxWorkers = runtime.NumCPU() * 2
			}
		case Resources.GPU:
			opts.MaxWorkers = concurrency * 2
		case Resources.TPU:
			opts.MaxWorkers = concurrency * 2
		default:
			opts.MaxWorkers = concurrency * 2
		}
	}
	if opts.MaxWorkers < opts.MinWorkers {
		opts.MaxWorkers = opts.MinWorkers
	}

	if concurrency < opts.MinWorkers {
		concurrency = opts.MinWorkers
	}
	if concurrency > opts.MaxWorkers {
		concurrency = opts.MaxWorkers
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		options:       opts,
		concurrency:   concurrency,
		workers:       make([]*workerInfo, 0, concurrency),
		ctx:           ctx,
		cancel:        cancel,
		monitorStop:   make(chan struct{}),
		lastLowLoadAt: time.Time{},
		redis:         redis, // <<< injected redis client (may be nil)
	}

	if opts.LogChannel != nil {
		log := func(prefix, msg string) {
			if opts.NonBlockingLogs {
				select {
				case opts.LogChannel <- fmt.Sprintf("%s: %s", prefix, msg):
				default:
				}
			} else {
				opts.LogChannel <- fmt.Sprintf("%s: %s", prefix, msg)
			}
		}
		log("DEVICE", fmt.Sprintf("Selected %s", bestDevice.String()))
		for _, ii := range info {
			log("INFO", ii)
		}
		for _, ww := range warn {
			log("WARN", ww)
		}
		for _, ee := range errs {
			if ee != nil {
				log("ERROR", ee.Error())
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wp.addWorkerLocked()
	}

	if opts.AutoScale {
		go wp.monitorAutoscale()
	}
	return wp
}

func (wp *WorkerPool) addWorkerLocked() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	id := len(wp.workers)
	w := &workerInfo{
		id:   id,
		load: 0,
		stop: make(chan struct{}),
		wg:   &wp.wg,
	}
	w.cond = sync.NewCond(&w.mu)
	heap.Init(&w.taskQueue)
	wp.workers = append(wp.workers, w)
	wp.wg.Add(1)
	go w.startWorker(wp)
	wp.concurrency = len(wp.workers)
}

func (wp *WorkerPool) AddWorker() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.options != nil && wp.options.MaxWorkers > 0 && len(wp.workers) >= wp.options.MaxWorkers {
		return errors.New("max workers reached")
	}
	wp.addWorkerLocked()
	return nil
}

func (wp *WorkerPool) removeOneIdleWorker() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if len(wp.workers) <= wp.options.MinWorkers {
		return false
	}

	for i := len(wp.workers) - 1; i >= 0; i-- {
		w := wp.workers[i]
		w.mu.Lock()
		isIdle := w.load == 0 && w.taskQueue.Len() == 0 && !w.closing
		if isIdle {
			w.closing = true
			close(w.stop)
			w.cond.Broadcast()
			wp.workers = append(wp.workers[:i], wp.workers[i+1:]...)
			w.mu.Unlock()
			wp.concurrency = len(wp.workers)
			return true
		}
		w.mu.Unlock()
	}
	return false
}

func (wp *WorkerPool) monitorAutoscale() {
	ticker := time.NewTicker(wp.options.EvalInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			wp.evaluateScaling()
		case <-wp.monitorStop:
			return
		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) evaluateScaling() {
	wp.mu.Lock()
	workersSnapshot := make([]*workerInfo, len(wp.workers))
	copy(workersSnapshot, wp.workers)
	wp.mu.Unlock()
	totalQueue := 0
	totalLoad := 0
	for _, w := range workersSnapshot {
		w.mu.Lock()
		q := w.taskQueue.Len()
		totalQueue += q
		totalLoad += w.load
		w.mu.Unlock()
	}
	nWorkers := len(workersSnapshot)
	if nWorkers == 0 {
		return
	}
	avgLoad := float64(totalLoad) / float64(nWorkers)
	backlogPerWorker := float64(totalQueue) / float64(nWorkers)
	if backlogPerWorker > wp.options.ScaleUpThreshold {
		// try add one
		if wp.options.MaxWorkers > 0 && nWorkers >= wp.options.MaxWorkers {
			wp.safeLog("Autoscale: at max workers; cannot scale up")
			return
		}
		err := wp.AddWorker()
		if err == nil {
			wp.safeLog(fmt.Sprintf("Autoscale: scaled up to %d workers (backlog %.2f > %.2f)", len(wp.workers), backlogPerWorker, wp.options.ScaleUpThreshold))
		} else {
			wp.safeLog(fmt.Sprintf("Autoscale: scale up failed: %v", err))
		}
		wp.lastLowLoadAt = time.Time{}
		return
	}

	if avgLoad < wp.options.ScaleDownThreshold && backlogPerWorker < wp.options.ScaleDownThreshold {
		if wp.lastLowLoadAt.IsZero() {
			wp.lastLowLoadAt = time.Now()
			return
		}
		if time.Since(wp.lastLowLoadAt) >= wp.options.IdleGracePeriod {
			removed := wp.removeOneIdleWorker()
			if removed {
				wp.safeLog(fmt.Sprintf("Autoscale: scaled down to %d workers (avg load %.2f)", len(wp.workers), avgLoad))
			} else {
				wp.safeLog("Autoscale: no idle worker to remove")
			}
		}
	} else {
		wp.lastLowLoadAt = time.Time{}
	}
}

func (w *workerInfo) startWorker(wp *WorkerPool) {
	defer w.wg.Done()
	for {
		w.mu.Lock()
		for w.taskQueue.Len() == 0 {
			select {
			case <-w.stop:
				w.mu.Unlock()
				return
			default:
			}
			select {
			case <-wp.ctx.Done():
				w.mu.Unlock()
				return
			default:
			}
			w.cond.Wait()
		}
		item := heap.Pop(&w.taskQueue)
		var task *Task
		if item != nil {
			task = item.(*Task)
		}
		w.mu.Unlock()

		if task == nil {
			continue
		}
		wp.executeTask(w, task)
	}
}

func (wp *WorkerPool) executeTask(w *workerInfo, task *Task) {
	w.mu.Lock()
	w.load += task.Weight
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.load -= task.Weight
		if w.load < 0 {
			w.load = 0
		}
		w.mu.Unlock()
		defer func() { recover() }()
		close(task.ResultCh)
	}()

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in task %d: %v", task.ID, r)
			res := WorkerResult{Result: nil, Info: nil, Warnings: nil, Errors: []error{err}}
			wp.safeLog(fmt.Sprintf("PANIC task %d: %v", task.ID, r))
			select {
			case task.ResultCh <- res:
			default:
			}
			if wp.options != nil && wp.options.OnTaskFinish != nil {
				wp.options.OnTaskFinish(task.ID, res)
			}
		}
	}()

	if wp.options != nil && wp.options.OnTaskStart != nil {
		wp.options.OnTaskStart(task.ID)
	}

	maxRetries := 0
	timeout := wp.options.Timeout
	if wp.options != nil {
		maxRetries = wp.options.MaxRetries
		if wp.options.Timeout > 0 {
			timeout = wp.options.Timeout
		}
	}
	retries := 0

	var res interface{}
	var info []string
	var warn []string
	var errs []error

	for {
		ctx := wp.ctx
		ctx, cancel := context.WithTimeout(ctx, timeout)
		res, info, warn, errs = task.Func(ctx)
		cancel()
		for _, m := range info {
			wp.safeLog(fmt.Sprintf("INFO task %d: %s", task.ID, m))
		}
		for _, m := range warn {
			wp.safeLog(fmt.Sprintf("WARN task %d: %s", task.ID, m))
		}
		for _, e := range errs {
			if e != nil {
				wp.safeLog(fmt.Sprintf("ERROR task %d: %v", task.ID, e))
			}
		}

		if len(errs) == 0 {
			break
		}
		retries++
		if retries > maxRetries {
			break
		}
		wp.safeLog(fmt.Sprintf("Retrying task %d (%d/%d)", task.ID, retries, maxRetries))
	}

	result := WorkerResult{Result: res, Info: info, Warnings: warn, Errors: errs}
	select {
	case task.ResultCh <- result:
	default:
		select {
		case task.ResultCh <- result:
		case <-time.After(100 * time.Millisecond):
			wp.safeLog(fmt.Sprintf("Dropping result for task %d: receiver not ready", task.ID))
		}
	}
	if wp.options != nil && wp.options.OnTaskFinish != nil {
		wp.options.OnTaskFinish(task.ID, result)
	}

	// ---- Redis writes: write per-task status + short-lived worker load ----
	if wp.redis != nil {
		ctx := context.Background()
		status := "ok"
		if len(errs) > 0 {
			status = "error"
		}
		_ = wp.redis.SetValue(ctx, fmt.Sprintf("task:%d:status", task.ID), status, 24*time.Hour)

		// write worker load (short TTL) for monitoring dashboards
		_ = wp.redis.SetValue(ctx, fmt.Sprintf("worker:%d:load", w.id), w.load, 30*time.Second)
	}

}

func (wp *WorkerPool) safeLog(msg string) {
	if wp.options == nil || wp.options.LogChannel == nil {
		return
	}
	if wp.options.NonBlockingLogs {
		select {
		case wp.options.LogChannel <- msg:
		default:
		}
	} else {
		wp.options.LogChannel <- msg
	}
}

func (wp *WorkerPool) SubmitTask(f TaskFunc, p TaskPriority, weight int) (int64, <-chan WorkerResult, error) {
	if f == nil {
		return 0, nil, errors.New("task func cannot be nil")
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.workers) == 0 {
		return 0, nil, errors.New("no workers available")
	}

	id := atomic.AddInt64(&wp.nextTaskID, 1)
	resultCh := make(chan WorkerResult, 1)
	task := &Task{
		ID:       id,
		Func:     f,
		Priority: p,
		ResultCh: resultCh,
		Weight:   weight,
		created:  time.Now(),
	}

	// NOTE:
	// If you want pre-schedule deduplication (skip scheduling when work already done),
	// you need a caller-provided unique key for the unit-of-work. We can add an
	// optional RunOptions.TaskKey func(...) string that returns a dedupe key. For now
	// we only write results to Redis after execution (avoids false negatives).

	// --- FIXED WORKER SELECTION (Round-Robin) ---
	if len(wp.workers) == 0 {
		return 0, nil, errors.New("no workers available")
	}

	// advance round-robin pointer
	wp.rrIndex = (wp.rrIndex + 1) % len(wp.workers)
	selected := wp.workers[wp.rrIndex]

	if selected == nil {
		return 0, nil, errors.New("failed to select worker")
	}

	selected.mu.Lock()
	heap.Push(&selected.taskQueue, task)
	selected.cond.Signal()
	selected.mu.Unlock()


	return id, resultCh, nil
}

func (wp *WorkerPool) UpdateTaskPriority(taskID int64, newPriority TaskPriority) bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for _, w := range wp.workers {
		w.mu.Lock()
		for _, t := range w.taskQueue {
			if t.ID == taskID {
				t.Priority = newPriority
				heap.Fix(&w.taskQueue, t.index)
				w.mu.Unlock()
				return true
			}
		}
		w.mu.Unlock()
	}
	return false
}

func (wp *WorkerPool) GetLoads() []int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	out := make([]int, len(wp.workers))
	for i, w := range wp.workers {
		w.mu.Lock()
		out[i] = w.load
		w.mu.Unlock()
	}
	return out
}

func (wp *WorkerPool) WorkerCount() int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return len(wp.workers)
}

func (wp *WorkerPool) Stop() {
	close(wp.monitorStop)
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
	wp.mu.Unlock()
	wp.wg.Wait()
}

func (wp *WorkerPool) Status() (int, int) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	active := len(wp.workers)
	queued := 0
	for _, w := range wp.workers {
		w.mu.Lock()
		queued += w.taskQueue.Len()
		w.mu.Unlock()
	}
	return active, queued
}
