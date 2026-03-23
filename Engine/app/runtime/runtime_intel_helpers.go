// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"strings"
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
	if p.generated == nil {
		return defaultGeneratedMeta()
	}
	meta, ok := p.generated[strings.ToLower(strings.TrimSpace(domain))]
	if !ok {
		return defaultGeneratedMeta()
	}
	return normalizeGeneratedMeta(meta)
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
