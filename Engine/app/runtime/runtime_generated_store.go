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
	resolveIntervalHours int,
) (int64, *generatedDomainSpool, error) {
	if err := resetGeneratedDomainQueue(store); err != nil {
		return 0, nil, err
	}
	spoolPathSeed := resolveSpoolPathSeed(path, generatedOutput)
	spool, err := newGeneratedDomainSpool(spoolPathSeed)
	if err != nil {
		return 0, nil, err
	}
	pending, err := prepareGeneratedSpoolForRun(
		ctx,
		spool,
		path,
		modules,
		generatedOutput,
		resolveIntervalHours,
	)
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

func resolveSpoolPathSeed(keywordsPath, generatedOutput string) string {
	_ = keywordsPath
	return generatedOutput
}

func prepareGeneratedSpoolForRun(
	ctx context.Context,
	spool *generatedDomainSpool,
	path string,
	modules ModuleFactory,
	generatedOutput string,
	resolveIntervalHours int,
) (int64, error) {
	count, err := spool.ensureDataset(ctx, path, modules)
	if err != nil || count == 0 {
		return count, err
	}
	if err := spool.syncDoneFromGeneratedOutput(ctx, generatedOutput); err != nil {
		return 0, err
	}
	pending, err := spool.pendingCount(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	lastCycleUnix, err := spool.loadLastCycleUnix(ctx)
	if err != nil {
		return 0, err
	}
	due := cycleDue(lastCycleUnix, now, resolveIntervalFromConfig(resolveIntervalHours))
	if due && pending == 0 {
		if err := spool.resetResolveCycle(ctx); err != nil {
			return 0, err
		}
		pending = count
	}
	if err := spool.prepareRun(ctx); err != nil {
		return 0, err
	}
	if shouldMarkCycleStart(lastCycleUnix, due, pending) {
		if err := spool.markResolveCycleStarted(ctx, now); err != nil {
			return 0, err
		}
	}
	return pending, nil
}

func resolveIntervalFromConfig(hours int) time.Duration {
	if hours <= 0 {
		hours = 6
	}
	return time.Duration(hours) * time.Hour
}

func cycleDue(lastCycleUnix int64, now time.Time, interval time.Duration) bool {
	if interval <= 0 || lastCycleUnix <= 0 {
		return true
	}
	return now.Sub(time.Unix(lastCycleUnix, 0)) >= interval
}

func shouldMarkCycleStart(lastCycleUnix int64, due bool, pending int64) bool {
	if pending <= 0 {
		return false
	}
	if lastCycleUnix <= 0 {
		return true
	}
	return due
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

func loadGeneratedMeta(
	ctx context.Context,
	store intelQueueStore,
	spool *generatedDomainSpool,
	domain string,
) (generatedDomainMeta, error) {
	domain = normalizeGeneratedDomainName(domain)
	if domain == "" {
		return defaultGeneratedMeta(), nil
	}
	meta, found, cacheErr := loadGeneratedMetaFromCache(ctx, store, domain)
	if cacheErr == nil && found {
		return meta, nil
	}
	spoolMeta, spoolFound, spoolErr := loadGeneratedMetaFromSpool(ctx, spool, domain)
	if spoolErr != nil {
		return defaultGeneratedMeta(), errors.Join(cacheErr, spoolErr)
	}
	if spoolFound {
		return spoolMeta, nil
	}
	if cacheErr != nil {
		return defaultGeneratedMeta(), cacheErr
	}
	return defaultGeneratedMeta(), nil
}

func loadGeneratedMetaFromCache(
	ctx context.Context,
	store intelQueueStore,
	domain string,
) (generatedDomainMeta, bool, error) {
	ioCtx, cancel := context.WithTimeout(ctx, generatedStoreIOTime)
	defer cancel()
	raw, err := store.Eval(ioCtx, readGeneratedMetaScript, []string{generatedMetaKey(domain)})
	if err != nil {
		return defaultGeneratedMeta(), false, err
	}
	meta, found := parseGeneratedMeta(raw)
	return meta, found, nil
}

func loadGeneratedMetaFromSpool(
	ctx context.Context,
	spool *generatedDomainSpool,
	domain string,
) (generatedDomainMeta, bool, error) {
	if spool == nil {
		return defaultGeneratedMeta(), false, nil
	}
	ioCtx, cancel := context.WithTimeout(ctx, generatedStoreIOTime)
	defer cancel()
	return spool.metaByDomain(ioCtx, domain)
}

func parseGeneratedMeta(raw interface{}) (generatedDomainMeta, bool) {
	meta := defaultGeneratedMeta()
	values, ok := raw.([]interface{})
	if !ok || len(values) < 3 || metaFieldsMissing(values[:3]) {
		return meta, false
	}
	if score, ok := parseFloatString(stringValue(values[0])); ok {
		meta.RiskScore = score
	}
	if value := stringValue(values[1]); value != "" {
		meta.Confidence = value
	}
	if value := stringValue(values[2]); value != "" {
		meta.GeneratedBy = value
	}
	return normalizeGeneratedMeta(meta), true
}

func metaFieldsMissing(values []interface{}) bool {
	for _, value := range values {
		if value != nil {
			return false
		}
	}
	return true
}

func parseFloatString(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return ""
	}
}
