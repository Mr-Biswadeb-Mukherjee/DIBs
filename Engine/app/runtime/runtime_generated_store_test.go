package runtime

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeGeneratedModuleFactory struct {
	items       []GeneratedDomain
	streamCalls *int
}

func (f fakeGeneratedModuleFactory) NewResolver(Config, ModuleLogger) DNSResolver {
	return nil
}

func (f fakeGeneratedModuleFactory) StreamGeneratedDomains(_ string, sink GeneratedDomainSink) error {
	if f.streamCalls != nil {
		*f.streamCalls = *f.streamCalls + 1
	}
	for _, item := range f.items {
		if err := sink(item); err != nil {
			return err
		}
	}
	return nil
}

func (f fakeGeneratedModuleFactory) NewDNSIntelService(int64) DNSIntelService {
	return nil
}

type fakeGeneratedCacheStore struct {
	queue []string
	meta  map[string][]interface{}
}

func newFakeGeneratedCacheStore() *fakeGeneratedCacheStore {
	return &fakeGeneratedCacheStore{
		queue: make([]string, 0, 8),
		meta:  make(map[string][]interface{}),
	}
}

func (s *fakeGeneratedCacheStore) Eval(
	_ context.Context,
	script string,
	keys []string,
	args ...interface{},
) (interface{}, error) {
	if script == storeGeneratedDomainScript {
		return s.evalStore(keys, args), nil
	}
	if script == readGeneratedMetaScript {
		return s.evalRead(keys), nil
	}
	return nil, nil
}

func (s *fakeGeneratedCacheStore) evalStore(keys []string, args []interface{}) interface{} {
	if len(keys) < 2 || len(args) < 4 {
		return int64(1)
	}
	metaKey := strings.TrimSpace(keys[0])
	queueValue := strings.TrimSpace(stringValue(args[0]))
	s.meta[metaKey] = []interface{}{
		stringValue(args[1]),
		stringValue(args[2]),
		stringValue(args[3]),
	}
	s.queue = append([]string{queueValue}, s.queue...)
	return int64(1)
}

func (s *fakeGeneratedCacheStore) evalRead(keys []string) interface{} {
	if len(keys) == 0 {
		return []interface{}{nil, nil, nil}
	}
	values, ok := s.meta[strings.TrimSpace(keys[0])]
	if !ok {
		return []interface{}{nil, nil, nil}
	}
	return values
}

func (s *fakeGeneratedCacheStore) GetValue(context.Context, string) (string, error) {
	return "", nil
}

func (s *fakeGeneratedCacheStore) SetValue(context.Context, string, interface{}, time.Duration) error {
	return nil
}

func (s *fakeGeneratedCacheStore) Delete(_ context.Context, key string) error {
	if key == generatedDomainQueueKey {
		s.queue = s.queue[:0]
	}
	return nil
}

func (s *fakeGeneratedCacheStore) RPush(context.Context, string, ...string) error {
	return nil
}

func (s *fakeGeneratedCacheStore) BLPop(
	_ context.Context,
	_ time.Duration,
	_ string,
) (string, bool, error) {
	if len(s.queue) == 0 {
		return "", false, nil
	}
	value := s.queue[0]
	s.queue = s.queue[1:]
	return value, true, nil
}

func (s *fakeGeneratedCacheStore) Expire(context.Context, string, time.Duration) error {
	return nil
}

func (s *fakeGeneratedCacheStore) Close() error {
	return nil
}

func (s *fakeGeneratedCacheStore) clearMeta() {
	s.meta = make(map[string][]interface{})
}

func TestStreamGeneratedDomainsToSpoolStagesAndPops(t *testing.T) {
	modules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{
			{Domain: "alpha.example", RiskScore: 0.55, Confidence: "high", GeneratedBy: "mutation"},
			{Domain: "beta.example", RiskScore: 0.12, Confidence: "low", GeneratedBy: "dga"},
			{Domain: "gamma.example", RiskScore: 0.77, Confidence: "medium", GeneratedBy: "combo"},
		},
	}
	store := newFakeGeneratedCacheStore()
	outputPath := t.TempDir() + "/Generated_Domain.ndjson"

	total, spool, err := streamGeneratedDomainsToSpool(
		context.Background(),
		"keywords.csv",
		modules,
		store,
		outputPath,
	)
	if err != nil {
		t.Fatalf("streamGeneratedDomainsToSpool error: %v", err)
	}
	defer func() { _ = spool.Close() }()

	if total != 3 || spool == nil {
		t.Fatalf("unexpected spool state: total=%d spool_nil=%v", total, spool == nil)
	}

	got := popAllDomains(t, store, spool, 3)
	assertPoppedDomains(t, got)

	_, ok, err := popGeneratedDomain(context.Background(), store, spool)
	if err != nil || ok {
		t.Fatalf("expected empty queue after all pops, ok=%v err=%v", ok, err)
	}
}

func popAllDomains(
	t *testing.T,
	store *fakeGeneratedCacheStore,
	spool *generatedDomainSpool,
	n int,
) map[string]bool {
	t.Helper()
	got := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		domain, ok, err := popGeneratedDomain(context.Background(), store, spool)
		if err != nil || !ok {
			t.Fatalf("popGeneratedDomain failed at %d: ok=%v err=%v", i, ok, err)
		}
		got[domain] = true
	}
	return got
}

func assertPoppedDomains(t *testing.T, got map[string]bool) {
	t.Helper()
	for _, domain := range []string{"alpha.example", "beta.example", "gamma.example"} {
		if !got[domain] {
			t.Fatalf("expected popped domain %q, got=%v", domain, got)
		}
	}
}

func TestLoadGeneratedMetaFallsBackToSpool(t *testing.T) {
	modules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{
			{Domain: "cache-miss.example", RiskScore: 0.88, Confidence: "high", GeneratedBy: "mutation"},
		},
	}
	store := newFakeGeneratedCacheStore()
	outputPath := t.TempDir() + "/Generated_Domain.ndjson"

	_, spool, err := streamGeneratedDomainsToSpool(context.Background(), "", modules, store, outputPath)
	if err != nil {
		t.Fatalf("streamGeneratedDomainsToSpool error: %v", err)
	}
	defer func() { _ = spool.Close() }()

	store.clearMeta()
	meta, err := loadGeneratedMeta(context.Background(), store, spool, " cache-miss.example ")
	if err != nil {
		t.Fatalf("loadGeneratedMeta fallback error: %v", err)
	}
	if meta.RiskScore != 0.88 || meta.Confidence != "high" || meta.GeneratedBy != "mutation" {
		t.Fatalf("unexpected fallback meta: %#v", meta)
	}
}
