// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	generatedSpoolFileName     = "generated_domains.spool.sqlite3"
	generatedBatchSize         = 200
	generatedInsertBatchSize   = 1000
	generatedSpoolMetaTable    = "generated_spool_meta"
	generatedSpoolSignatureKey = "keywords_signature"
	generatedSpoolLastCycleKey = "last_resolve_cycle_unix"
)

type spooledGeneratedDomain struct {
	id   int64
	item GeneratedDomain
}

type generatedDomainSpool struct {
	db        *sql.DB
	batchSize int
	mu        sync.Mutex
}

func newGeneratedDomainSpool(keywordsPath string) (*generatedDomainSpool, error) {
	dbPath := generatedSpoolPath(keywordsPath)
	if err := ensureSpoolDir(dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open generated spool db: %w", err)
	}

	spool := &generatedDomainSpool{db: db, batchSize: generatedBatchSize}
	if err := spool.prepare(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return spool, nil
}

func generatedSpoolPath(keywordsPath string) string {
	clean := strings.TrimSpace(keywordsPath)
	dir := filepath.Dir(clean)
	if strings.EqualFold(filepath.Base(dir), "Output") {
		dir = filepath.Join(filepath.Dir(dir), "Input")
	}
	if dir == "" || dir == "." {
		dir = "."
	}
	return filepath.Join(dir, generatedSpoolFileName)
}

func ensureSpoolDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated spool directory: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) prepare() error {
	if s == nil || s.db == nil {
		return errors.New("generated spool db is not initialized")
	}
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	return initGeneratedSpoolSchema(s.db)
}

func initGeneratedSpoolSchema(db *sql.DB) error {
	createTableDDL := `
CREATE TABLE IF NOT EXISTS generated_domains (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  domain TEXT NOT NULL,
  risk_score REAL NOT NULL,
  confidence TEXT NOT NULL,
  generated_by TEXT NOT NULL,
  done INTEGER NOT NULL DEFAULT 0,
  queued INTEGER NOT NULL DEFAULT 0
);`
	if _, err := db.Exec(createTableDDL); err != nil {
		return fmt.Errorf("create generated spool schema: %w", err)
	}
	if err := ensureGeneratedSpoolMetaTable(db); err != nil {
		return err
	}
	if err := ensureGeneratedSpoolDoneColumn(db); err != nil {
		return err
	}
	return ensureGeneratedSpoolIndexes(db)
}

func ensureGeneratedSpoolMetaTable(db *sql.DB) error {
	stmt := `
CREATE TABLE IF NOT EXISTS generated_spool_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);`
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("create generated spool meta table: %w", err)
	}
	return nil
}

func ensureGeneratedSpoolDoneColumn(db *sql.DB) error {
	_, err := db.Exec("ALTER TABLE generated_domains ADD COLUMN done INTEGER NOT NULL DEFAULT 0")
	if err == nil || isDuplicateColumnError(err) {
		return nil
	}
	return fmt.Errorf("ensure generated spool done column: %w", err)
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func ensureGeneratedSpoolIndexes(db *sql.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_generated_domains_queued_id ON generated_domains(queued, id)",
		"CREATE INDEX IF NOT EXISTS idx_generated_domains_domain ON generated_domains(domain)",
		"CREATE INDEX IF NOT EXISTS idx_generated_domains_done_queued_id ON generated_domains(done, queued, id)",
	}
	for _, stmt := range indexes {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create generated spool index: %w", err)
		}
	}
	return nil
}

func (s *generatedDomainSpool) Close() error {
	if s == nil {
		return nil
	}
	return closeSpoolDB(s.db)
}

func closeSpoolDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}

func generatedInputSignature(path string) (string, error) {
	hasher := sha256.New()
	cleanPath := strings.TrimSpace(path)
	if _, err := io.WriteString(hasher, "keywords_path:"+cleanPath+"\n"); err != nil {
		return "", err
	}
	if cleanPath == "" {
		return hex.EncodeToString(hasher.Sum(nil)), nil
	}
	if err := appendFileHash(hasher, cleanPath); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func appendFileHash(hasher io.Writer, path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		_, writeErr := io.WriteString(hasher, "keywords_file:missing")
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("open keywords file for signature: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash keywords file: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) syncDatasetSignature(
	ctx context.Context,
	path string,
) error {
	signature, err := generatedInputSignature(path)
	if err != nil {
		return err
	}
	if err := s.resetLegacyOrMismatchedDataset(ctx, signature); err != nil {
		return err
	}
	return s.upsertSignature(ctx, signature)
}

func (s *generatedDomainSpool) resetLegacyOrMismatchedDataset(
	ctx context.Context,
	signature string,
) error {
	count, err := s.rowCount(ctx)
	if err != nil || count == 0 {
		return err
	}
	stored, err := s.loadSignature(ctx)
	if err != nil {
		return err
	}
	if stored != "" && stored == signature {
		return nil
	}
	return s.clearDataset(ctx)
}

func (s *generatedDomainSpool) loadSignature(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("generated spool db is not initialized")
	}
	row := s.db.QueryRowContext(
		ctx,
		"SELECT value FROM generated_spool_meta WHERE key = ?",
		generatedSpoolSignatureKey,
	)
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read generated spool signature: %w", err)
	}
	return strings.TrimSpace(value), nil
}

func (s *generatedDomainSpool) loadLastCycleUnix(ctx context.Context) (int64, error) {
	value, err := s.loadMetaValue(ctx, generatedSpoolLastCycleKey)
	if err != nil || value == "" {
		return 0, err
	}
	unixTs, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil {
		return 0, nil
	}
	return unixTs, nil
}

func (s *generatedDomainSpool) loadMetaValue(ctx context.Context, key string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("generated spool db is not initialized")
	}
	row := s.db.QueryRowContext(ctx, "SELECT value FROM generated_spool_meta WHERE key = ?", strings.TrimSpace(key))
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read generated spool meta key=%s: %w", key, err)
	}
	return strings.TrimSpace(value), nil
}

func (s *generatedDomainSpool) upsertSignature(ctx context.Context, value string) error {
	if s == nil || s.db == nil {
		return errors.New("generated spool db is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO generated_spool_meta(key, value)
VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, generatedSpoolSignatureKey, strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("upsert generated spool signature: %w", err)
	}
	return nil
}

func (s *generatedDomainSpool) upsertLastCycleUnix(ctx context.Context, unixTs int64) error {
	return s.upsertMetaValue(ctx, generatedSpoolLastCycleKey, strconv.FormatInt(unixTs, 10))
}

func (s *generatedDomainSpool) upsertMetaValue(ctx context.Context, key, value string) error {
	if s == nil || s.db == nil {
		return errors.New("generated spool db is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO generated_spool_meta(key, value)
VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, strings.TrimSpace(key), strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("upsert generated spool meta key=%s: %w", key, err)
	}
	return nil
}
