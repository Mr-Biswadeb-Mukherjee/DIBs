// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"time"
)

func closeWriters(writers ...RecordWriter) error {
	var errs []error
	for _, writer := range writers {
		if writer == nil {
			continue
		}
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *intelPipeline) generatedMeta(domain string) generatedDomainMeta {
	if p == nil {
		return defaultGeneratedMeta()
	}
	meta, err := loadGeneratedMeta(p.ctx, p.store, p.generated, domain)
	if err != nil {
		if p.logErr != nil {
			p.logErr("generated-meta-read", domain, err)
		}
		return defaultGeneratedMeta()
	}
	return normalizeGeneratedMeta(meta)
}

func (p *intelPipeline) markGeneratedDone(domain string) {
	if p == nil || p.generated == nil {
		return
	}
	domain = normalizeGeneratedDomainName(domain)
	if domain == "" {
		return
	}
	ioCtx, cancel := context.WithTimeout(p.ctx, generatedStoreIOTime)
	defer cancel()
	if err := p.generated.markDone(ioCtx, domain); err != nil && p.logErr != nil {
		p.logErr("generated-spool-mark-done", domain, err)
	}
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

func newDNSIntelWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.DNSIntelOutput, writers, "dns-intel-writer", logErr)
}

func newGeneratedDomainWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.GeneratedOutput, writers, "generated-domain-writer", logErr)
}

func newResolvedDomainWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.ResolvedOutput, writers, "resolved-domain-writer", logErr)
}

func newNDJSONWriter(
	path string,
	writers WriterFactory,
	module string,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	opts := WriterOptions{
		BatchSize:  300,
		FlushEvery: time.Second,
		LogHooks: WriterLogHooks{
			OnError: func(err error) {
				if logErr != nil {
					logErr(module, "write", err)
				}
			},
		},
	}
	return writers.NewNDJSONWriter(path, opts)
}

func resetIntelQueue(store intelQueueStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), intelQueueIOTime)
	defer cancel()
	return store.Delete(ctx, intelQueueKey)
}
