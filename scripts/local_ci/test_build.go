package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runTestStage() error {
	fmt.Println("==> test")
	if err := configureRedisForCI(); err != nil {
		return err
	}

	out, err := runCmd(25*time.Minute, "go", "test", "./...", "-v", "-race", "-count=1")
	if writeErr := writeText("test_output.txt", out); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return err
	}

	if _, err := runCmd(20*time.Minute, "go", "test", "./...", "-coverpkg=./...", "-coverprofile=coverage.out", "-count=1"); err != nil {
		return err
	}

	coverageFunc, err := runCmd(1*time.Minute, "go", "tool", "cover", "-func=coverage.out")
	if writeErr := writeText("coverage_func.txt", coverageFunc); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return err
	}

	gate, err := applyCoverageGate(coverageFunc)
	if writeErr := writeText("coverage_gate.txt", gate); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return err
	}
	if gate == "BLOCKED" {
		return errors.New("coverage gate blocked")
	}
	return nil
}

func configureRedisForCI() error {
	content := "host: \"127.0.0.1\"\nport: 6379\nusername: \"\"\npassword: \"\"\ndb: 0\nmax_retries: 3\npool_size: 20\nmin_idle_conns: 5\ncluster: false\naddrs: []\nprefix: \"\"\ndial_timeout: 5\nread_timeout: 5\nwrite_timeout: 5\nhealth_tick: 10\nbackoff_max: 20\n"
	return writeText("Setting/redis.yaml", content)
}

func applyCoverageGate(report string) (string, error) {
	raw, err := extractCoverage(report)
	if err != nil {
		return "UNKNOWN", err
	}

	penalty := readPenalty()
	final := raw - penalty
	fmt.Printf("Coverage raw %.2f%%, penalty %.2f%%, final %.2f%%\n", raw, penalty, final)

	if final < coverageQAWarn {
		fmt.Printf("ERROR: Coverage %.2f%% is below %.2f%%\n", final, coverageQAWarn)
		return "BLOCKED", nil
	}
	if final < coverageBuild {
		fmt.Printf("ERROR: Coverage %.2f%% is below %.2f%%\n", final, coverageBuild)
		return "BLOCKED", nil
	}
	if final < coveragePass {
		fmt.Printf("WARN: Coverage %.2f%% is below %.2f%%\n", final, coveragePass)
		return "WARN", nil
	}
	return "PASS", nil
}

func extractCoverage(report string) (float64, error) {
	for _, line := range strings.Split(report, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "total:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			break
		}
		return strconv.ParseFloat(strings.TrimSuffix(parts[2], "%"), 64)
	}
	return 0, errors.New("unable to parse total coverage")
}

func readPenalty() float64 {
	content, err := os.ReadFile("arch_score.txt")
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
	if err != nil {
		return 0
	}
	return v
}

func runBuildStage() error {
	fmt.Println("==> build")
	targets := []buildTarget{
		{"linux", "amd64", ""},
		{"linux", "arm64", ""},
		{"windows", "amd64", ".exe"},
		{"darwin", "amd64", ""},
		{"darwin", "arm64", ""},
	}
	if err := os.MkdirAll("dist", 0o755); err != nil {
		return err
	}
	for _, t := range targets {
		name := fmt.Sprintf("infermal-%s-%s%s", t.goos, t.goarch, t.suffix)
		env := []string{"GOOS=" + t.goos, "GOARCH=" + t.goarch, "CGO_ENABLED=0"}
		if _, err := runCmdWithEnv(10*time.Minute, env, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join("dist", name), "./main.go"); err != nil {
			return err
		}
	}
	return nil
}
