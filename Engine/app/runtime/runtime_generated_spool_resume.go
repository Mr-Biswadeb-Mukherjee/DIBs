// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const generatedDoneSyncBatchSize = 500

type generatedOutputLine struct {
	Domain string `json:"domain"`
}

func (s *generatedDomainSpool) syncDoneFromGeneratedOutput(
	ctx context.Context,
	outputPath string,
) error {
	file, err := openGeneratedOutput(outputPath)
	if err != nil {
		return err
	}
	if file == nil {
		return nil
	}
	defer file.Close()

	domains, scanErr := parseGeneratedOutputDomains(ctx, file)
	if scanErr != nil {
		return scanErr
	}
	return s.markDoneDomains(ctx, domains)
}

func openGeneratedOutput(path string) (*os.File, error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err == nil {
		return file, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, fmt.Errorf("open generated output file: %w", err)
}

func parseGeneratedOutputDomains(ctx context.Context, file *os.File) ([]string, error) {
	scanner := newGeneratedOutputScanner(file)
	domains := make([]string, 0, generatedDoneSyncBatchSize)
	seen := make(map[string]struct{}, generatedDoneSyncBatchSize)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		domain := parseOutputDomain(scanner.Bytes())
		if domain == "" {
			continue
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan generated output file: %w", err)
	}
	return domains, nil
}

func newGeneratedOutputScanner(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

func parseOutputDomain(line []byte) string {
	if len(line) == 0 {
		return ""
	}
	var row generatedOutputLine
	if err := json.Unmarshal(line, &row); err != nil {
		return ""
	}
	return normalizeGeneratedDomainName(row.Domain)
}

func (s *generatedDomainSpool) markDoneDomains(ctx context.Context, domains []string) error {
	if len(domains) == 0 || s == nil || s.db == nil {
		return nil
	}
	tx, stmt, err := openMarkDoneTx(ctx, s)
	if err != nil {
		return err
	}
	runErr := executeMarkDoneChunk(ctx, tx, stmt, domains)
	closeErr := closeMarkDoneTx(stmt, tx, runErr == nil)
	if runErr != nil {
		return runErr
	}
	return closeErr
}

func openMarkDoneTx(ctx context.Context, s *generatedDomainSpool) (*sql.Tx, *sql.Stmt, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin done-sync tx: %w", err)
	}
	if err := createDoneSyncTempTable(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO done_sync_domains(domain) VALUES(?)")
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, fmt.Errorf("prepare done-sync stmt: %w", err)
	}
	return tx, stmt, nil
}

func createDoneSyncTempTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TEMP TABLE IF NOT EXISTS done_sync_domains (
  domain TEXT PRIMARY KEY
)`)
	if err != nil {
		return fmt.Errorf("create done-sync temp table: %w", err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM done_sync_domains")
	if err != nil {
		return fmt.Errorf("reset done-sync temp table: %w", err)
	}
	return nil
}

func executeMarkDoneChunk(ctx context.Context, tx *sql.Tx, stmt *sql.Stmt, domains []string) error {
	for _, domain := range domains {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, domain); err != nil {
			return fmt.Errorf("stage done-sync domain: %w", err)
		}
	}
	if tx == nil {
		return errors.New("done-sync tx is unavailable")
	}
	if err := applyDoneSyncUpdate(ctx, tx); err != nil {
		return err
	}
	return nil
}

func applyDoneSyncUpdate(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
UPDATE generated_domains
SET done = 1
WHERE done = 0
  AND domain IN (SELECT domain FROM done_sync_domains)
`)
	if err != nil {
		return fmt.Errorf("apply done-sync update: %w", err)
	}
	return nil
}

func closeMarkDoneTx(stmt *sql.Stmt, tx *sql.Tx, commit bool) error {
	stmtErr := stmt.Close()
	if commit {
		return errors.Join(stmtErr, tx.Commit())
	}
	return errors.Join(stmtErr, rollbackUnlessDone(tx))
}
