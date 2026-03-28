// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	privateKeyEnv         = "INFERMAL_API_PRIVATE_KEY"
	privateKeyPathEnv     = "INFERMAL_API_PRIVATE_KEY_PATH"
	defaultPrivateKeyPath = "Setting/api_private.key"

	privateKeyLabelPrefix = "prvkey_"
	publicKeyLabelPrefix  = "pubkey_"
	keyAlgorithm          = "ed25519"
	publicKeyNamespace    = "Infermal_v2"
)

var errInvalidPrivateKey = errors.New("invalid private api key")

type APIKeyPair struct {
	Private string
	Public  string
}

func NewAPIKeyPairFromPrivate(privateKey string) (APIKeyPair, error) {
	normalized, key, err := parsePrivateKey(privateKey)
	if err != nil {
		return APIKeyPair{}, err
	}
	pub := key.Public().(ed25519.PublicKey)
	return APIKeyPair{
		Private: normalized,
		Public:  formatPublicKey(pub),
	}, nil
}

func GenerateAPIKeyPair() (APIKeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return APIKeyPair{}, err
	}
	return APIKeyPair{
		Private: formatPrivateKey(priv),
		Public:  formatPublicKey(pub),
	}, nil
}

func BuildAPIKeyPair(privateKey string) (APIKeyPair, error) {
	if strings.TrimSpace(privateKey) == "" {
		return GenerateAPIKeyPair()
	}
	return NewAPIKeyPairFromPrivate(privateKey)
}

func LoadOrCreateAPIKeyPair(privateKey, privateKeyPath string) (APIKeyPair, error) {
	if strings.TrimSpace(privateKey) != "" {
		return NewAPIKeyPairFromPrivate(privateKey)
	}
	path, err := resolvePrivateKeyPath(privateKeyPath)
	if err != nil {
		return APIKeyPair{}, err
	}
	pair, err := loadPairFromStore(path)
	if err == nil {
		return pair, nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errInvalidPrivateKey) {
		return createPairAndStore(path)
	}
	return APIKeyPair{}, err
}

func PrivateKeyEnv() string {
	return privateKeyEnv
}

func PrivateKeyPathEnv() string {
	return privateKeyPathEnv
}

func DefaultPrivateKeyPath() string {
	return defaultPrivateKeyPath
}

func (k APIKeyPair) ValidatePublic(provided string) bool {
	expected := strings.TrimSpace(k.Public)
	value := strings.TrimSpace(provided)
	if expected == "" || len(value) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func parsePrivateKey(privateKey string) (string, ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(privateKey)
	if trimmed == "" {
		return "", nil, wrapInvalidPrivate(errors.New("private key is empty"))
	}
	if key, normalized, handled, err := parseLabeledPrivateKey(trimmed); handled {
		if err != nil {
			return "", nil, err
		}
		return normalized, key, nil
	}
	key, err := decodePrivateMaterial(trimmed)
	if err != nil {
		return "", nil, err
	}
	return formatPrivateKey(key), key, nil
}

func parseLabeledPrivateKey(privateKey string) (ed25519.PrivateKey, string, bool, error) {
	label := privateKeyLabelPrefix + keyAlgorithm
	if !strings.HasPrefix(privateKey, label) {
		return nil, "", false, nil
	}
	fields := strings.Fields(privateKey)
	if len(fields) != 3 {
		return nil, "", true, wrapInvalidPrivate(errors.New("private key fields are malformed"))
	}
	if fields[0] != label || fields[2] != publicKeyNamespace {
		return nil, "", true, wrapInvalidPrivate(errors.New("private key label or namespace mismatch"))
	}
	key, err := decodePrivateMaterial(fields[1])
	if err != nil {
		return nil, "", true, err
	}
	return key, formatPrivateKey(key), true, nil
}

func decodePrivateMaterial(encoded string) (ed25519.PrivateKey, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, wrapInvalidPrivate(err)
	}
	switch len(decoded) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	default:
		msg := fmt.Sprintf("unsupported private key length: %d", len(decoded))
		return nil, wrapInvalidPrivate(errors.New(msg))
	}
}

func formatPrivateKey(key ed25519.PrivateKey) string {
	label := privateKeyLabelPrefix + keyAlgorithm
	token := base64.RawURLEncoding.EncodeToString(key)
	return fmt.Sprintf("%s %s %s", label, token, publicKeyNamespace)
}

func formatPublicKey(key ed25519.PublicKey) string {
	label := publicKeyLabelPrefix + keyAlgorithm
	token := base64.RawURLEncoding.EncodeToString(key)
	return fmt.Sprintf("%s %s %s", label, token, publicKeyNamespace)
}

func wrapInvalidPrivate(err error) error {
	if err == nil {
		return errInvalidPrivateKey
	}
	return fmt.Errorf("%w: %v", errInvalidPrivateKey, err)
}

func resolvePrivateKeyPath(path string) (string, error) {
	return normalizeSettingFilePath(path)
}

func loadPairFromStore(path string) (APIKeyPair, error) {
	privateKey, err := readPrivateKey(path)
	if err != nil {
		return APIKeyPair{}, err
	}
	return NewAPIKeyPairFromPrivate(privateKey)
}

func readPrivateKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	privateKey := strings.TrimSpace(string(content))
	if privateKey == "" {
		return "", wrapInvalidPrivate(errors.New("stored private key is empty"))
	}
	return privateKey, nil
}

func createPairAndStore(path string) (APIKeyPair, error) {
	pair, err := GenerateAPIKeyPair()
	if err != nil {
		return APIKeyPair{}, err
	}
	if err := writePrivateKey(path, pair.Private); err != nil {
		return APIKeyPair{}, err
	}
	return pair, nil
}

func writePrivateKey(path, privateKey string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data := []byte(strings.TrimSpace(privateKey) + "\n")
	return os.WriteFile(path, data, 0o600)
}
