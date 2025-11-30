package worker

import (
	"context"
	"sync"
	"time"
)

// ------------------------------------------------------------
// Task Function Types
// ------------------------------------------------------------

type TaskFunc func(ctx context.Context) (interface{}, []string, []string, []error)

// Unified result returned to callers and stored in Redis (optional)
type WorkerResult struct {
	Result   interface{}
	Info     []string
	Warnings []string
	Errors   []error
}

// ------------------------------------------------------------
// Priority & Task Configuration
// ------------------------------------------------------------

type TaskPriority int

const (
	Low TaskPriority = iota
	Medium
	High
)

// Options controlling behavior of the worker system, applicable
// to both local and Redis-backed implementations.
type RunOptions struct {
	Timeout         time.Duration
	MaxRetries      int
	LogChannel      chan string
	OnTaskStart     func(taskID int64)
	OnTaskFinish    func(taskID int64, res WorkerResult)
	NonBlockingLogs bool

	// Autoscaling settings (optional for redis-based workers)
	AutoScale          bool
	MinWorkers         int
	MaxWorkers         int
	ScaleUpThreshold   float64
	ScaleDownThreshold float64
	IdleGracePeriod    time.Duration
	EvalInterval       time.Duration

	// Deterministic dedupe key generator
	TaskKey func(f TaskFunc) string

	// Local dedupe TTL: expired inflight entries are purged.
	// Default set in NewWorkerPool if zero.
	DedupeTTL time.Duration
}

// ------------------------------------------------------------
// Task Definition
// ------------------------------------------------------------

// A task represents a unit of execution. Local worker pool uses
// Weight for load tracking and index for priority queue heap index.
// Redis-backed mode stores metadata in Redis (no local heap usage).
type Task struct {
	ID       int64
	Func     TaskFunc
	Priority TaskPriority
	ResultCh chan WorkerResult
	Weight   int
	Created  time.Time

	// Internal fields for local pool
	index  int
	Dedupe string
}

// ------------------------------------------------------------
// Priority Queue (local mode only)
// ------------------------------------------------------------

// PriorityQueue supports priority-based scheduling inside the
// local worker implementation. Redis workers do not use this.
type PriorityQueue []*Task

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].Created.Before(pq[j].Created)
	}
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index, pq[j].index = i, j
}

func (pq *PriorityQueue) Push(x interface{}) {
	t := x.(*Task)
	t.index = len(*pq)
	*pq = append(*pq, t)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	t := old[n-1]
	old[n-1], t.index = nil, -1
	*pq = old[:n-1]
	return t
}

// ------------------------------------------------------------
// Redis Storage Interface
// ------------------------------------------------------------

// Allows plugging in any Redis client without binding to a specific library.
// Each backend must implement GET/SET with TTL.
type RedisStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
}

// ------------------------------------------------------------
// Worker Structures
// ------------------------------------------------------------

// Per-worker state used in local mode. Redis-based workers do
// not maintain an in-memory queue and use this only for minimal
// load tracking or optional features.
type workerInfo struct {
	ID   int
	Load int

	mu   sync.Mutex
	cond *sync.Cond

	taskQueue PriorityQueue
	stop      chan struct{}
	wg        *sync.WaitGroup
	closing   bool
}

// inflightEntry represents a local dedupe registry entry.
type inflightEntry struct {
	id      int64
	ch      chan WorkerResult
	created time.Time
}

// WorkerPool represents either a local or Redis-backed pool.
// The actual scheduling logic is implemented in worker.go or
// worker_local.go depending on the mode.
type WorkerPool struct {
	mu      sync.Mutex
	workers []*workerInfo
	options *RunOptions

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	nextID  int64
	rrIndex int

	inflight map[string]inflightEntry // dedupe registry for local mode
	redis    RedisStore

	monitorStop   chan struct{}
	lastLowLoadAt time.Time
}

const (
	DefaultTimeout      = 10 * time.Second
	DefaultScaleUp      = 1.2
	DefaultScaleDown    = 0.3
	DefaultIdleGrace    = 10 * time.Second
	DefaultEvalInterval = 3 * time.Second
)
