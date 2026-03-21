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

func closeWriters(writers ...*filewriter.NDJSONWriter) error {
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
	return newNDJSONWriter("Output/DNS_Intel.ndjson", "dns-intel-writer", logErr)
}

func newGeneratedDomainWriter(logErr moduleErrorLogger) (*filewriter.NDJSONWriter, error) {
	if err := os.MkdirAll("Output", 0o755); err != nil {
		return nil, err
	}
	return newNDJSONWriter("Output/Generated_Domains.ndjson", "generated-domain-writer", logErr)
}

func newNDJSONWriter(path, module string, logErr moduleErrorLogger) (*filewriter.NDJSONWriter, error) {
	opts := filewriter.NDJSONOptions{
		BatchSize:  300,
		FlushEvery: time.Second,
		LogHooks: filewriter.LogHooks{
			OnError: func(err error) {
				if logErr != nil {
					logErr(module, "write", err)
				}
			},
		},
	}
	return filewriter.NewNDJSONWriter(path, opts)
}

func resetIntelQueue(store intelQueueStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), intelQueueIOTime)
	defer cancel()
	return store.Delete(ctx, intelQueueKey)
}
