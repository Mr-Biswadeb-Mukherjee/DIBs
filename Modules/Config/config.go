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
	TimeoutSeconds   int
	MaxRetries       int
	AutoScale        bool
}

var defaultConfig = Config{
	RateLimit:        130,
	CooldownAfter:    500,
	CooldownDuration: 30,
	TimeoutSeconds:   4,
	MaxRetries:       0,
	AutoScale:        true,
}

func LoadOrCreateConfig(path string) (Config, error) {
	_ = os.MkdirAll("Setting", 0o755)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := writeDefault(path)
		if err != nil {
			return defaultConfig, err
		}
		return defaultConfig, nil
	}

	return parseConfig(path)
}

func writeDefault(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf(
		"rate_limit=%d\ncooldown_after=%d\ncooldown_duration=%d\ntimeout_seconds=%d\nmax_retries=%d\nautoscale=%t\n",
		defaultConfig.RateLimit,
		defaultConfig.CooldownAfter,
		defaultConfig.CooldownDuration,
		defaultConfig.TimeoutSeconds,
		defaultConfig.MaxRetries,
		defaultConfig.AutoScale,
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

		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key, value := pair[0], pair[1]

		switch key {
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
		}
	}

	return cfg, scanner.Err()
}
