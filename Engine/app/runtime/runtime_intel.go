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
}

type intelPipeline struct {
	store        intelQueueStore
	service      DNSIntelService
	writer       RecordWriter
	domainWriter RecordWriter
	generated    map[string]generatedDomainMeta

	ctx    context.Context
	cancel context.CancelFunc
	done   chan error
	logErr moduleErrorLogger
	onDone func()
}

func newIntelPipeline(
	parentCtx context.Context,
	store intelQueueStore,
	service DNSIntelService,
	writerFactory WriterFactory,
	paths Paths,
	generated map[string]generatedDomainMeta,
	logErr moduleErrorLogger,
	onDone func(),
) (*intelPipeline, error) {
	if service == nil {
		return nil, errors.New("dns intel service is required")
	}

	writer, domainWriter, err := newPipelineWriters(writerFactory, paths, logErr)
	if err != nil {
		return nil, err
	}
	if err := resetIntelQueue(store); err != nil {
		_ = closeWriters(writer, domainWriter)
		return nil, err
	}

	ctx, cancel := context.WithCancel(parentCtx)
	p := &intelPipeline{
		store:        store,
		service:      service,
		writer:       writer,
		domainWriter: domainWriter,
		generated:    generated,
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan error, 1),
		logErr:       logErr,
		onDone:       onDone,
	}
	go p.consumeLoop()
	return p, nil
}

func newPipelineWriters(
	writerFactory WriterFactory,
	paths Paths,
	logErr moduleErrorLogger,
) (RecordWriter, RecordWriter, error) {
	writer, err := newDNSIntelWriter(paths, writerFactory, logErr)
	if err != nil {
		return nil, nil, err
	}
	domainWriter, err := newGeneratedDomainWriter(paths, writerFactory, logErr)
	if err != nil {
		_ = writer.Close()
		return nil, nil, err
	}
	return writer, domainWriter, nil
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
	p.domainWriter.WriteRecord(unresolvedDomainRecord(domain, p.generatedMeta(domain)))
	return true
}

func (p *intelPipeline) WriteResolvedFallback(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	p.domainWriter.WriteRecord(resolvedDomainRecord(domain, p.generatedMeta(domain)))
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

	closeErr := closeWriters(p.writer, p.domainWriter)
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
		p.domainWriter.WriteRecord(resolvedDomainRecord(rec.Domain, p.generatedMeta(rec.Domain)))
	}
}
