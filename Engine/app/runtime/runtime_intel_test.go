package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type intelTestStore struct {
	mu    sync.Mutex
	queue []string
}

func newIntelTestStore() *intelTestStore {
	return &intelTestStore{
		queue: make([]string, 0, 16),
	}
}

func (s *intelTestStore) Delete(context.Context, string) error {
	s.mu.Lock()
	s.queue = s.queue[:0]
	s.mu.Unlock()
	return nil
}

func (s *intelTestStore) RPush(_ context.Context, _ string, values ...string) error {
	s.mu.Lock()
	s.queue = append(s.queue, values...)
	s.mu.Unlock()
	return nil
}

func (s *intelTestStore) BLPop(ctx context.Context, timeout time.Duration, _ string) (string, bool, error) {
	timer := time.NewTimer(timeout)
	tick := time.NewTicker(2 * time.Millisecond)
	defer timer.Stop()
	defer tick.Stop()

	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			value := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return value, true, nil
		}
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case <-timer.C:
			return "", false, nil
		case <-tick.C:
		}
	}
}

func (s *intelTestStore) Expire(context.Context, string, time.Duration) error {
	return nil
}

func (s *intelTestStore) Eval(context.Context, string, []string, ...interface{}) (interface{}, error) {
	return []interface{}{nil, nil, nil}, nil
}

type intelTestWriter struct {
	mu      sync.Mutex
	records []interface{}
}

func newIntelTestWriter() *intelTestWriter {
	return &intelTestWriter{
		records: make([]interface{}, 0, 16),
	}
}

func (w *intelTestWriter) WriteRecord(record interface{}) {
	w.mu.Lock()
	w.records = append(w.records, record)
	w.mu.Unlock()
}

func (w *intelTestWriter) Close() error {
	return nil
}

func (w *intelTestWriter) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.records)
}

type slowIntelService struct {
	delay time.Duration
	calls int64
}

func (s *slowIntelService) Run(ctx context.Context, domains []IntelDomain) ([]IntelRecord, error) {
	atomic.AddInt64(&s.calls, 1)
	timer := time.NewTimer(s.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}

	if len(domains) == 0 {
		return nil, nil
	}
	return []IntelRecord{{Domain: domains[0].Name}}, nil
}

func TestIntelPipelineDrainsResolvedQueueAfterParentCancel(t *testing.T) {
	parentCtx, cancelParent := context.WithCancel(context.Background())
	store := newIntelTestStore()
	service := &slowIntelService{delay: 250 * time.Millisecond}

	dnsWriter := newIntelTestWriter()
	generatedWriter := newIntelTestWriter()
	resolvedWriter := newIntelTestWriter()
	clusterWriter := newIntelTestWriter()

	pipe := buildIntelPipeline(
		parentCtx,
		store,
		nil,
		service,
		nil,
		dnsWriter,
		generatedWriter,
		resolvedWriter,
		clusterWriter,
		nil,
		nil,
	)
	go pipe.consumeLoop()

	const total = 12
	for i := 0; i < total; i++ {
		domain := fmt.Sprintf("d-%d.example", i)
		if ok := pipe.EnqueueResolved(domain); !ok {
			t.Fatalf("enqueue failed for %q", domain)
		}
	}

	cancelParent()
	if err := pipe.StopAndWait(); err != nil {
		t.Fatalf("StopAndWait error: %v", err)
	}

	if got := resolvedWriter.Count(); got != total {
		t.Fatalf("resolved writer lost records: got=%d want=%d", got, total)
	}
	if got := generatedWriter.Count(); got != total {
		t.Fatalf("generated writer lost records: got=%d want=%d", got, total)
	}
	if got := atomic.LoadInt64(&service.calls); got > 1 {
		t.Fatalf("expected fast fallback drain after cancellation, service calls=%d", got)
	}
}
