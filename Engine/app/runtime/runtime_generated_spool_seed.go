// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *generatedDomainSpool) ensureDataset(
	ctx context.Context,
	path string,
	modules ModuleFactory,
) (int64, error) {
	if err := s.syncDatasetSignature(ctx, path); err != nil {
		return 0, err
	}
	count, err := s.rowCount(ctx)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return count, nil
	}
	return s.ingest(ctx, path, modules)
}

func (s *generatedDomainSpool) prepareRun(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("generated spool db is not initialized")
	}
	_, err := s.db.ExecContext(ctx, "UPDATE generated_domains SET queued = 0 WHERE done = 0 AND queued <> 0")
	if err != nil {
		return fmt.Errorf("reset generated spool run state: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) rowCount(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("generated spool db is not initialized")
	}
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM generated_domains")
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count generated spool rows: %w", err)
	}
	return count, nil
}

func (s *generatedDomainSpool) pendingCount(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("generated spool db is not initialized")
	}
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM generated_domains WHERE done = 0")
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count generated spool pending rows: %w", err)
	}
	return count, nil
}

func (s *generatedDomainSpool) ingest(
	ctx context.Context,
	path string,
	modules ModuleFactory,
) (int64, error) {
	writer, err := newSpoolBatchWriter(ctx, s.db, generatedInsertBatchSize)
	if err != nil {
		return 0, err
	}
	count, streamErr := streamDomainsToBatchWriter(path, modules, writer)
	closeErr := writer.close(streamErr == nil)
	if streamErr == nil && closeErr == nil {
		return count, nil
	}
	clearErr := s.clearDataset(ctx)
	return 0, errors.Join(streamErr, closeErr, clearErr)
}

func streamDomainsToBatchWriter(
	path string,
	modules ModuleFactory,
	writer *spoolBatchWriter,
) (int64, error) {
	var count int64
	err := modules.StreamGeneratedDomains(path, func(item GeneratedDomain) error {
		inserted, err := writer.insert(item)
		if err != nil {
			return err
		}
		if inserted {
			count++
		}
		return nil
	})
	return count, err
}

func (s *generatedDomainSpool) clearDataset(ctx context.Context) error {
	_, rowsErr := s.db.ExecContext(ctx, "DELETE FROM generated_domains")
	if rowsErr != nil {
		return fmt.Errorf("clear generated spool dataset: %w", rowsErr)
	}
	_, metaErr := s.db.ExecContext(ctx, "DELETE FROM "+generatedSpoolMetaTable)
	if metaErr != nil {
		return fmt.Errorf("clear generated spool metadata: %w", metaErr)
	}
	return nil
}

type spoolBatchWriter struct {
	ctx       context.Context
	db        *sql.DB
	batchSize int
	pending   int
	tx        *sql.Tx
	stmt      *sql.Stmt
}

func newSpoolBatchWriter(
	ctx context.Context,
	db *sql.DB,
	batchSize int,
) (*spoolBatchWriter, error) {
	writer := &spoolBatchWriter{ctx: ctx, db: db, batchSize: batchSize}
	if err := writer.openBatch(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *spoolBatchWriter) openBatch() error {
	tx, err := w.db.BeginTx(w.ctx, nil)
	if err != nil {
		return fmt.Errorf("begin generated spool tx: %w", err)
	}
	stmt, err := tx.PrepareContext(w.ctx, `
INSERT INTO generated_domains(domain, risk_score, confidence, generated_by, queued)
VALUES(?, ?, ?, ?, 0)
`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare generated spool insert: %w", err)
	}
	w.tx = tx
	w.stmt = stmt
	w.pending = 0
	return nil
}

func (w *spoolBatchWriter) insert(item GeneratedDomain) (bool, error) {
	domain := normalizeGeneratedDomainName(item.Domain)
	if domain == "" {
		return false, nil
	}
	meta := normalizeGeneratedMeta(generatedDomainMeta{
		RiskScore:   item.RiskScore,
		Confidence:  item.Confidence,
		GeneratedBy: item.GeneratedBy,
	})
	if _, err := w.stmt.ExecContext(w.ctx, domain, meta.RiskScore, meta.Confidence, meta.GeneratedBy); err != nil {
		return false, fmt.Errorf("insert generated spool row: %w", err)
	}
	w.pending++
	return true, w.rotateIfNeeded()
}

func (w *spoolBatchWriter) rotateIfNeeded() error {
	if w.pending < w.batchSize {
		return nil
	}
	if err := w.close(true); err != nil {
		return err
	}
	return w.openBatch()
}

func (w *spoolBatchWriter) close(commit bool) error {
	stmtErr := w.closeStmt()
	if w.tx == nil {
		return stmtErr
	}

	var txErr error
	if commit {
		txErr = w.tx.Commit()
	} else {
		txErr = rollbackUnlessDone(w.tx)
	}
	w.tx = nil
	w.pending = 0
	return errors.Join(stmtErr, txErr)
}

func (w *spoolBatchWriter) closeStmt() error {
	if w.stmt == nil {
		return nil
	}
	err := w.stmt.Close()
	w.stmt = nil
	return err
}

func rollbackUnlessDone(tx *sql.Tx) error {
	if tx == nil {
		return nil
	}
	err := tx.Rollback()
	if err == nil || errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}
