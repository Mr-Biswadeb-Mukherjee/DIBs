// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *generatedDomainSpool) loadNextBatch(ctx context.Context, store CacheStore) (int, error) {
	if s == nil {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.reserveRows(ctx)
	if err != nil || len(rows) == 0 {
		return len(rows), err
	}
	pushed, err := pushSpoolRowsToCache(ctx, store, rows)
	if err == nil {
		return pushed, nil
	}
	if pushed < len(rows) {
		requeueErr := s.releaseRows(ctx, rows[pushed:])
		return pushed, errors.Join(err, requeueErr)
	}
	return pushed, err
}

func (s *generatedDomainSpool) reserveRows(ctx context.Context) ([]spooledGeneratedDomain, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin generated batch tx: %w", err)
	}
	rows, err := fetchUnqueuedRows(ctx, tx, s.batchSize)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if len(rows) == 0 {
		_ = tx.Rollback()
		return nil, nil
	}
	if err := markRowsQueued(ctx, tx, rows); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit generated batch tx: %w", err)
	}
	return rows, nil
}

func fetchUnqueuedRows(ctx context.Context, tx *sql.Tx, limit int) ([]spooledGeneratedDomain, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, domain, risk_score, confidence, generated_by
FROM generated_domains
WHERE done = 0 AND queued = 0
ORDER BY id
LIMIT ?
`, limit)
	if err != nil {
		return nil, fmt.Errorf("query generated batch: %w", err)
	}
	defer rows.Close()
	return scanSpooledRows(rows)
}

func scanSpooledRows(rows *sql.Rows) ([]spooledGeneratedDomain, error) {
	out := make([]spooledGeneratedDomain, 0, generatedBatchSize)
	for rows.Next() {
		item, err := scanSpooledRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan generated batch: %w", err)
	}
	return out, nil
}

func scanSpooledRow(rows *sql.Rows) (spooledGeneratedDomain, error) {
	var row spooledGeneratedDomain
	if err := rows.Scan(
		&row.id,
		&row.item.Domain,
		&row.item.RiskScore,
		&row.item.Confidence,
		&row.item.GeneratedBy,
	); err != nil {
		return spooledGeneratedDomain{}, fmt.Errorf("scan generated row: %w", err)
	}
	return row, nil
}

func markRowsQueued(ctx context.Context, tx *sql.Tx, rows []spooledGeneratedDomain) error {
	placeholders, args := rowIDArgs(rows)
	query := "UPDATE generated_domains SET queued = 1 WHERE id IN (" + placeholders + ")"
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mark generated batch queued: %w", err)
	}
	return nil
}

func rowIDArgs(rows []spooledGeneratedDomain) (string, []interface{}) {
	args := make([]interface{}, 0, len(rows))
	tokens := make([]string, 0, len(rows))
	for _, row := range rows {
		tokens = append(tokens, "?")
		args = append(args, row.id)
	}
	return strings.Join(tokens, ","), args
}

func pushSpoolRowsToCache(
	ctx context.Context,
	store CacheStore,
	rows []spooledGeneratedDomain,
) (int, error) {
	for idx, row := range rows {
		if err := storeGeneratedDomain(ctx, store, row.item); err != nil {
			return idx, err
		}
	}
	return len(rows), nil
}

func (s *generatedDomainSpool) releaseRows(ctx context.Context, rows []spooledGeneratedDomain) error {
	if len(rows) == 0 {
		return nil
	}
	placeholders, args := rowIDArgs(rows)
	query := "UPDATE generated_domains SET queued = 0 WHERE id IN (" + placeholders + ")"
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("release generated batch rows: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) markDone(ctx context.Context, domain string) error {
	domain = normalizeGeneratedDomainName(domain)
	if domain == "" || s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "UPDATE generated_domains SET done = 1 WHERE domain = ? AND done = 0", domain)
	if err != nil {
		return fmt.Errorf("mark generated spool row done: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) metaByDomain(
	ctx context.Context,
	domain string,
) (generatedDomainMeta, bool, error) {
	if s == nil || s.db == nil {
		return defaultGeneratedMeta(), false, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT risk_score, confidence, generated_by
FROM generated_domains
WHERE domain = ?
ORDER BY id DESC
LIMIT 1
`, normalizeGeneratedDomainName(domain))
	var meta generatedDomainMeta
	if err := row.Scan(&meta.RiskScore, &meta.Confidence, &meta.GeneratedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return defaultGeneratedMeta(), false, nil
		}
		return defaultGeneratedMeta(), false, fmt.Errorf("read generated metadata from spool: %w", err)
	}
	return normalizeGeneratedMeta(meta), true, nil
}
