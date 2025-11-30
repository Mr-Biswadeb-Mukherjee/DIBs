package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	RateLimit        int
	CooldownAfter    int
	CooldownDuration int
	TimeoutSeconds   int // worker timeout
	MaxRetries       int // worker retries
	AutoScale        bool

	// DNS settings
	UpstreamDNS  string
	BackupDNS    string
	DNSRetries   int
	DNSTimeoutMS int64
}

var defaultConfig = Config{
	// Worker / Performance defaults
	RateLimit:        40,
	CooldownAfter:    2500,
	CooldownDuration: 20,
	TimeoutSeconds:   5,
	MaxRetries:       3,
	AutoScale:        false,

	// DNS defaults
	UpstreamDNS:  "9.9.9.9:53",
	BackupDNS:    "1.1.1.1:53",
	DNSRetries:   2,
	DNSTimeoutMS: 500,
}

func LoadOrCreateConfig(path string) (Config, error) {
	_ = os.MkdirAll("Setting", 0o755)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create new config file with default values
		err := writeDefault(path)
		if err != nil {
			return defaultConfig, err
		}
		return defaultConfig, nil
	}

	// Parse existing config
	return parseConfig(path)
}

func writeDefault(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf(
		"# Worker / Performance\n"+
			"rate_limit=%d\n"+
			"cooldown_after=%d\n"+
			"cooldown_duration=%d\n"+
			"timeout_seconds=%d\n"+
			"max_retries=%d\n"+
			"autoscale=%t\n\n"+
			"# DNS settings\n"+
			"upstream_dns=%s\n"+
			"backup_dns=%s\n"+
			"dns_retries=%d\n"+
			"dns_timeout_ms=%d\n",
		defaultConfig.RateLimit,
		defaultConfig.CooldownAfter,
		defaultConfig.CooldownDuration,
		defaultConfig.TimeoutSeconds,
		defaultConfig.MaxRetries,
		defaultConfig.AutoScale,
		defaultConfig.UpstreamDNS,
		defaultConfig.BackupDNS,
		defaultConfig.DNSRetries,
		defaultConfig.DNSTimeoutMS,
	))

	return err
}

func parseConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return defaultConfig, err
	}
	defer file.Close()

	cfg := defaultConfig
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {

		// Worker / Performance
		case "rate_limit":
			cfg.RateLimit, _ = strconv.Atoi(value)
		case "cooldown_after":
			cfg.CooldownAfter, _ = strconv.Atoi(value)
		case "cooldown_duration":
			cfg.CooldownDuration, _ = strconv.Atoi(value)
		case "timeout_seconds":
			cfg.TimeoutSeconds, _ = strconv.Atoi(value)
		case "max_retries":
			cfg.MaxRetries, _ = strconv.Atoi(value)
		case "autoscale":
			cfg.AutoScale = (value == "true")

		// DNS
		case "upstream_dns":
			cfg.UpstreamDNS = value
		case "backup_dns":
			cfg.BackupDNS = value
		case "dns_retries":
			cfg.DNSRetries, _ = strconv.Atoi(value)
		case "dns_timeout_ms":
			ms, _ := strconv.Atoi(value)
			cfg.DNSTimeoutMS = int64(ms)
		}
	}

	return cfg, scanner.Err()
}
