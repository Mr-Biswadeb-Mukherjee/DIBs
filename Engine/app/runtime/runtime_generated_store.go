// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	generatedDomainQueueKey = "dns:generated:queue"
	generatedMetaKeyPrefix  = "meta:"

	generatedMetaTTL       = 48 * time.Hour
	generatedQueueWait     = 1 * time.Second
	generatedStoreIOTime   = 400 * time.Millisecond
	generatedBatchLoadTime = 1500 * time.Millisecond
)

const storeGeneratedDomainScript = `
redis.call("HSET", KEYS[1],
  "risk_score", ARGV[2],
  "confidence", ARGV[3],
  "generated_by", ARGV[4])
redis.call("EXPIRE", KEYS[1], tonumber(ARGV[5]))
redis.call("LPUSH", KEYS[2], ARGV[1])
return 1
`

const readGeneratedMetaScript = `
return redis.call("HMGET", KEYS[1], "risk_score", "confidence", "generated_by")
`

func streamGeneratedDomainsToSpool(
	ctx context.Context,
	path string,
	modules ModuleFactory,
	store CacheStore,
	generatedOutput string,
) (int64, *generatedDomainSpool, error) {
	if err := resetGeneratedDomainQueue(store); err != nil {
		return 0, nil, err
	}
	spool, err := newGeneratedDomainSpool(generatedOutput)
	if err != nil {
		return 0, nil, err
	}
	pending, err := prepareGeneratedSpoolForRun(ctx, spool, path, modules, generatedOutput)
	if err != nil {
		_ = spool.Close()
		return 0, nil, err
	}
	if pending == 0 {
		_ = spool.Close()
		return 0, nil, nil
	}
	if err := primeGeneratedQueue(ctx, store, spool); err != nil {
		_ = spool.Close()
		return 0, nil, err
	}
	return pending, spool, nil
}

func prepareGeneratedSpoolForRun(
	ctx context.Context,
	spool *generatedDomainSpool,
	path string,
	modules ModuleFactory,
	generatedOutput string,
) (int64, error) {
	count, err := spool.ensureDataset(ctx, path, modules)
	if err != nil || count == 0 {
		return count, err
	}
	if err := spool.prepareRun(ctx); err != nil {
		return 0, err
	}
	if err := spool.syncDoneFromGeneratedOutput(ctx, generatedOutput); err != nil {
		return 0, err
	}
	return spool.pendingCount(ctx)
}

func primeGeneratedQueue(
	ctx context.Context,
	store CacheStore,
	spool *generatedDomainSpool,
) error {
	loaded, err := loadNextGeneratedBatch(ctx, store, spool)
	if err != nil {
		return err
	}
	if loaded <= 0 {
		return errors.New("generated spool is empty after ingest")
	}
	return nil
}

func storeGeneratedDomain(ctx context.Context, store CacheStore, item GeneratedDomain) error {
	domain := normalizeGeneratedDomainName(item.Domain)
	if domain == "" {
		return nil
	}
	meta := normalizeGeneratedMeta(generatedDomainMeta{
		RiskScore:   item.RiskScore,
		Confidence:  item.Confidence,
		GeneratedBy: item.GeneratedBy,
	})

	ioCtx, cancel := context.WithTimeout(ctx, generatedStoreIOTime)
	defer cancel()

	_, err := store.Eval(ioCtx, storeGeneratedDomainScript,
		[]string{generatedMetaKey(domain), generatedDomainQueueKey},
		domain,
		formatRiskScore(meta.RiskScore),
		meta.Confidence,
		meta.GeneratedBy,
		int64(generatedMetaTTL/time.Second),
	)
	return err
}

func resetGeneratedDomainQueue(store CacheStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), generatedStoreIOTime)
	defer cancel()
	return store.Delete(ctx, generatedDomainQueueKey)
}

func popGeneratedDomain(
	ctx context.Context,
	store CacheStore,
	spool *generatedDomainSpool,
) (string, bool, error) {
	value, ok, err := popGeneratedDomainFromCache(ctx, store)
	if err != nil || ok {
		return value, ok, err
	}
	if spool == nil {
		return "", false, nil
	}
	loaded, err := loadNextGeneratedBatch(ctx, store, spool)
	if err != nil || loaded == 0 {
		return "", false, err
	}
	return popGeneratedDomainFromCache(ctx, store)
}

func popGeneratedDomainFromCache(ctx context.Context, store CacheStore) (string, bool, error) {
	ioCtx, cancel := context.WithTimeout(ctx, generatedQueueWait+generatedStoreIOTime)
	defer cancel()
	value, ok, err := store.BLPop(ioCtx, generatedQueueWait, generatedDomainQueueKey)
	if err != nil {
		return "", false, err
	}
	return normalizeGeneratedDomainName(value), ok, nil
}

func loadNextGeneratedBatch(
	ctx context.Context,
	store CacheStore,
	spool *generatedDomainSpool,
) (int, error) {
	if spool == nil {
		return 0, nil
	}
	ioCtx, cancel := context.WithTimeout(ctx, generatedBatchLoadTime)
	defer cancel()
	return spool.loadNextBatch(ioCtx, store)
}

func generatedMetaKey(domain string) string {
	return generatedMetaKeyPrefix + normalizeGeneratedDomainName(domain)
}

func normalizeGeneratedDomainName(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

func formatRiskScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 2, 64)
}
