package mutation

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type moduleConfig struct {
	Enabled    bool
	Character  bool
	Seed       bool
	Hashchain  bool
	TargetTLDs []string
}

type configEntry struct {
	Key   string
	Value string
}

var mutationEntries = []configEntry{
	{Key: "mutation_enabled", Value: "true"},
	{Key: "mutation_character", Value: "true"},
	{Key: "mutation_seed", Value: "true"},
	{Key: "mutation_hashchain", Value: "true"},
	{Key: "mutation_target_tlds", Value: ".in"},
}

var defaultModuleConfig = moduleConfig{
	Enabled:    true,
	Character:  true,
	Seed:       true,
	Hashchain:  true,
	TargetTLDs: []string{".in"},
}

func loadModuleConfig(path string) (moduleConfig, error) {
	if err := ensureMutationConfig(path); err != nil {
		return defaultModuleConfig, err
	}

	file, err := os.Open(path)
	if err != nil {
		return defaultModuleConfig, fmt.Errorf("open settings: %w", err)
	}
	defer file.Close()

	cfg := defaultModuleConfig
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		applyConfigLine(&cfg, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return defaultModuleConfig, fmt.Errorf("scan settings: %w", err)
	}
	cfg.TargetTLDs = normalizeTLDs(cfg.TargetTLDs)
	return cfg, nil
}

func ensureMutationConfig(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createMutationConfig(path)
		}
		return fmt.Errorf("read settings: %w", err)
	}

	existing := parseExistingKeys(string(content))
	missing := missingEntries(existing)
	if len(missing) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append settings: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(buildMutationBlock(missing)); err != nil {
		return fmt.Errorf("write mutation settings: %w", err)
	}
	return nil
}

func createMutationConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir settings dir: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create settings: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(buildMutationBlock(mutationEntries)); err != nil {
		return fmt.Errorf("write settings defaults: %w", err)
	}
	return nil
}

func parseExistingKeys(content string) map[string]struct{} {
	keys := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		key, ok := parseLine(scanner.Text())
		if ok {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func missingEntries(existing map[string]struct{}) []configEntry {
	missing := make([]configEntry, 0, len(mutationEntries))
	for _, entry := range mutationEntries {
		if _, ok := existing[entry.Key]; !ok {
			missing = append(missing, entry)
		}
	}
	return missing
}

func buildMutationBlock(entries []configEntry) string {
	var b strings.Builder
	b.WriteString("\n# Mutation settings\n")
	for _, entry := range entries {
		b.WriteString(entry.Key)
		b.WriteString("=")
		b.WriteString(entry.Value)
		b.WriteString("\n")
	}
	return b.String()
}

func applyConfigLine(cfg *moduleConfig, line string) {
	key, value, ok := parseKV(line)
	if !ok {
		return
	}

	switch key {
	case "mutation_enabled":
		cfg.Enabled = parseBool(value, cfg.Enabled)
	case "mutation_character":
		cfg.Character = parseBool(value, cfg.Character)
	case "mutation_seed":
		cfg.Seed = parseBool(value, cfg.Seed)
	case "mutation_hashchain":
		cfg.Hashchain = parseBool(value, cfg.Hashchain)
	case "mutation_target_tlds":
		cfg.TargetTLDs = parseTLDs(value)
	}
}

func parseLine(line string) (string, bool) {
	key, _, ok := parseKV(line)
	return key, ok
}

func parseKV(line string) (string, string, bool) {
	raw := strings.TrimSpace(line)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", "", false
	}

	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseTLDs(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeTLD(strings.TrimSpace(part))
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeTLDs(values []string) []string {
	unique := make(map[string]struct{})
	for _, v := range values {
		normalized := normalizeTLD(v)
		if normalized != "" {
			unique[normalized] = struct{}{}
		}
	}

	if len(unique) == 0 {
		return []string{".in"}
	}

	out := make([]string, 0, len(unique))
	for tld := range unique {
		out = append(out, tld)
	}
	return out
}

func normalizeTLD(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, ".") {
		return trimmed
	}
	return "." + trimmed
}

func resolveSettingsPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return defaultSettingsPath
	}

	dir := wd
	for {
		if hasGoMod(dir) {
			return filepath.Join(dir, defaultSettingsPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return defaultSettingsPath
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
