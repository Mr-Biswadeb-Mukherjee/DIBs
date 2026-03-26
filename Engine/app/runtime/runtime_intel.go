// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	intelQueueKey    = "dns:intel:queue"
	intelStopMarker  = "__infermal_dns_intel_stop__"
	intelQueueTTL    = 20 * time.Minute
	intelQueueWait   = 1 * time.Second
	intelQueueIOTime = 400 * time.Millisecond
)

type intelQueueStore interface {
	Delete(ctx context.Context, key string) error
	RPush(ctx context.Context, key string, values ...string) error
	BLPop(ctx context.Context, timeout time.Duration, key string) (string, bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

type intelPipeline struct {
	store           intelQueueStore
	generated       *generatedDomainSpool
	service         DNSIntelService
	writer          RecordWriter
	generatedWriter RecordWriter
	resolvedWriter  RecordWriter

	ctx    context.Context
	cancel context.CancelFunc
	done   chan error
	logErr moduleErrorLogger
	onDone func()
}

func newIntelPipeline(
	parentCtx context.Context,
	store intelQueueStore,
	generated *generatedDomainSpool,
	service DNSIntelService,
	writerFactory WriterFactory,
	paths Paths,
	logErr moduleErrorLogger,
	onDone func(),
) (*intelPipeline, error) {
	if service == nil {
		return nil, errors.New("dns intel service is required")
	}

	writer, generatedWriter, resolvedWriter, err := newPipelineWriters(writerFactory, paths, logErr)
	if err != nil {
		return nil, err
	}
	if err := resetIntelQueue(store); err != nil {
		_ = closeWriters(writer, generatedWriter, resolvedWriter)
		return nil, err
	}

	p := buildIntelPipeline(
		parentCtx,
		store,
		generated,
		service,
		writer,
		generatedWriter,
		resolvedWriter,
		logErr,
		onDone,
	)
	go p.consumeLoop()
	return p, nil
}

func buildIntelPipeline(
	parentCtx context.Context,
	store intelQueueStore,
	generated *generatedDomainSpool,
	service DNSIntelService,
	writer RecordWriter,
	generatedWriter RecordWriter,
	resolvedWriter RecordWriter,
	logErr moduleErrorLogger,
	onDone func(),
) *intelPipeline {
	ctx, cancel := context.WithCancel(parentCtx)
	return &intelPipeline{
		store:           store,
		generated:       generated,
		service:         service,
		writer:          writer,
		generatedWriter: generatedWriter,
		resolvedWriter:  resolvedWriter,
		ctx:             ctx,
		cancel:          cancel,
		done:            make(chan error, 1),
		logErr:          logErr,
		onDone:          onDone,
	}
}

func newPipelineWriters(
	writerFactory WriterFactory,
	paths Paths,
	logErr moduleErrorLogger,
) (RecordWriter, RecordWriter, RecordWriter, error) {
	writer, err := newDNSIntelWriter(paths, writerFactory, logErr)
	if err != nil {
		return nil, nil, nil, err
	}
	generatedWriter, err := newGeneratedDomainWriter(paths, writerFactory, logErr)
	if err != nil {
		_ = writer.Close()
		return nil, nil, nil, err
	}
	resolvedWriter, err := newResolvedDomainWriter(paths, writerFactory, logErr)
	if err != nil {
		_ = closeWriters(writer, generatedWriter)
		return nil, nil, nil, err
	}
	return writer, generatedWriter, resolvedWriter, nil
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

func (p *intelPipeline) WriteUnresolved(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	p.generatedWriter.WriteRecord(unresolvedDomainRecord(domain, p.generatedMeta(domain)))
	p.markGeneratedDone(domain)
	return true
}

func (p *intelPipeline) WriteResolvedFallback(domain string) bool {
	return p.writeResolved(domain)
}

func (p *intelPipeline) writeResolved(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	record := resolvedDomainRecord(domain, p.generatedMeta(domain))
	p.resolvedWriter.WriteRecord(record)
	p.generatedWriter.WriteRecord(record)
	p.markGeneratedDone(record.Domain)
	return true
}

func (p *intelPipeline) StopAndWait() error {
	if p == nil {
		return nil
	}

	stopErr := p.pushValue(context.Background(), intelStopMarker)
	if stopErr != nil {
		p.cancel()
	}
	consumeErr := <-p.done
	p.cancel()

	closeErr := closeWriters(p.writer, p.generatedWriter, p.resolvedWriter)
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
		if value == intelStopMarker {
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

	records, err := p.service.Run(runCtx, []IntelDomain{{Name: domain}})
	if err != nil {
		p.logErr("dns-intel-run", domain, err)
		p.WriteResolvedFallback(domain)
		return
	}
	if len(records) == 0 {
		p.WriteResolvedFallback(domain)
		return
	}

	for _, rec := range records {
		p.writer.WriteRecord(intelRecordToNDJSON(rec))
		recordDomain := strings.TrimSpace(rec.Domain)
		if recordDomain == "" {
			recordDomain = domain
		}
		p.writeResolved(recordDomain)
	}
}
