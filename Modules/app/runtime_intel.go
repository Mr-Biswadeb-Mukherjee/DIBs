package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	filewriter "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/filewriter"
	"github.com/official-biswadeb941/Infermal_v2/Modules/app/intel"
)

const (
	intelQueueKey    = "dns:intel:queue"
	intelStopToken   = "__infermal_dns_intel_stop__"
	intelQueueTTL    = 20 * time.Minute
	intelQueueWait   = 1 * time.Second
	intelQueueIOTime = 400 * time.Millisecond
)

type intelQueueStore interface {
	Delete(ctx context.Context, key string) error
	RPush(ctx context.Context, key string, values ...string) error
	BLPop(ctx context.Context, timeout time.Duration, key string) (string, bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type intelPipeline struct {
	store   intelQueueStore
	service *intel.DNSIntelService
	writer  *filewriter.NDJSONWriter

	ctx    context.Context
	cancel context.CancelFunc
	done   chan error
	logErr moduleErrorLogger
	onDone func()
}

func newIntelPipeline(
	parentCtx context.Context,
	store intelQueueStore,
	dnsTimeoutMS int64,
	logErr moduleErrorLogger,
	onDone func(),
) (*intelPipeline, error) {
	writer, err := newDNSIntelWriter(logErr)
	if err != nil {
		return nil, err
	}
	if err := resetIntelQueue(store); err != nil {
		_ = writer.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(parentCtx)
	p := &intelPipeline{
		store:   store,
		service: newDNSIntelService(dnsTimeoutMS),
		writer:  writer,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan error, 1),
		logErr:  logErr,
		onDone:  onDone,
	}
	go p.consumeLoop()
	return p, nil
}

func (p *intelPipeline) EnqueueResolved(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	if err := p.pushValue(p.ctx, domain); err != nil {
		p.logErr("dns-intel-queue", domain, err)
		return false
	}
	return true
}

func (p *intelPipeline) StopAndWait() error {
	if p == nil {
		return nil
	}

	stopErr := p.pushValue(context.Background(), intelStopToken)
	if stopErr != nil {
		p.cancel()
	}
	consumeErr := <-p.done
	p.cancel()

	closeErr := p.writer.Close()
	cleanupErr := resetIntelQueue(p.store)
	return errors.Join(stopErr, consumeErr, closeErr, cleanupErr)
}

func (p *intelPipeline) consumeLoop() {
	for {
		value, ok, err := p.popValue()
		if err != nil {
			if p.ctx.Err() != nil {
				p.done <- nil
				return
			}
			p.logErr("dns-intel-queue-pop", "redis", err)
			continue
		}
		if !ok {
			continue
		}
		if value == intelStopToken {
			p.done <- nil
			return
		}
		p.processDomain(value)
		if p.onDone != nil {
			p.onDone()
		}
	}
}

func (p *intelPipeline) popValue() (string, bool, error) {
	ctx, cancel := context.WithTimeout(p.ctx, intelQueueWait+intelQueueIOTime)
	defer cancel()
	return p.store.BLPop(ctx, intelQueueWait, intelQueueKey)
}

func (p *intelPipeline) pushValue(parent context.Context, value string) error {
	ctx, cancel := context.WithTimeout(parent, intelQueueIOTime)
	defer cancel()
	if err := p.store.RPush(ctx, intelQueueKey, value); err != nil {
		return err
	}
	return p.store.Expire(ctx, intelQueueKey, intelQueueTTL)
}

func (p *intelPipeline) processDomain(domain string) {
	runCtx, cancel := context.WithTimeout(p.ctx, 8*time.Second)
	defer cancel()

	records, err := p.service.Run(runCtx, []intel.Domain{{Name: domain}})
	if err != nil {
		p.logErr("dns-intel-run", domain, err)
		return
	}
	for _, rec := range records {
		p.writer.WriteRecord(intelRecordToNDJSON(rec))
	}
}

func newDNSIntelService(dnsTimeoutMS int64) *intel.DNSIntelService {
	timeout := intelLookupTimeout(dnsTimeoutMS)
	return intel.NewDNSIntelService(newDNSIntelResolver(), nil, 1, timeout)
}

func intelLookupTimeout(dnsTimeoutMS int64) time.Duration {
	if dnsTimeoutMS <= 0 {
		return 3 * time.Second
	}
	timeout := time.Duration(dnsTimeoutMS) * 6 * time.Millisecond
	if timeout < 2*time.Second {
		return 2 * time.Second
	}
	if timeout > 8*time.Second {
		return 8 * time.Second
	}
	return timeout
}

func newDNSIntelWriter(logErr moduleErrorLogger) (*filewriter.NDJSONWriter, error) {
	if err := os.MkdirAll("Output", 0o755); err != nil {
		return nil, err
	}
	opts := filewriter.NDJSONOptions{
		BatchSize:  300,
		FlushEvery: time.Second,
		LogHooks: filewriter.LogHooks{
			OnError: func(err error) {
				if logErr != nil {
					logErr("dns-intel-writer", "write", err)
				}
			},
		},
	}
	return filewriter.NewNDJSONWriter("Output/DNS_Intel.ndjson", opts)
}

func resetIntelQueue(store intelQueueStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), intelQueueIOTime)
	defer cancel()
	return store.Delete(ctx, intelQueueKey)
}
