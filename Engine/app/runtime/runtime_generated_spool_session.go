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
	"strings"
)

const generatedSpoolSignatureKey = "keywords_signature"

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
