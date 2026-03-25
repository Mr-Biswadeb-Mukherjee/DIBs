package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	coveragePass   = 95.0
	coverageBuild  = 90.0
	coverageQAWarn = 80.0
	cycloThreshold = 10
	slocWarn       = 250
	slocFail       = 300
)

var codeTargets = []string{"main.go", "Engine", "core", "bootstrap", "scripts"}

type buildTarget struct {
	goos   string
	goarch string
	suffix string
}

func main() {
	stage := flag.String("stage", "all", "pipeline stage: quality|test|build|all")
	flag.Parse()

	if err := runPipeline(strings.ToLower(strings.TrimSpace(*stage))); err != nil {
		fmt.Fprintln(os.Stderr, "CI local run failed:", err)
		os.Exit(1)
	}
	fmt.Println("Local CI pipeline completed successfully.")
}

func runPipeline(stage string) error {
	switch stage {
	case "quality":
		return runQuality()
	case "test":
		return runTestStage()
	case "build":
		return runBuildStage()
	case "all":
		if err := runQuality(); err != nil {
			return err
		}
		if err := runTestStage(); err != nil {
			return err
		}
		return runBuildStage()
	default:
		return fmt.Errorf("unknown stage %q", stage)
	}
}

func runQuality() error {
	fmt.Println("==> quality")
	if err := runDependencyCheck(); err != nil {
		return err
	}
	if err := runFormatCheck(); err != nil {
		return err
	}
	if _, err := runCmd(5*time.Minute, "go", "vet", "./..."); err != nil {
		return err
	}
	if _, err := runCmd(15*time.Minute, "go", "run", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest", "run", "--timeout=5m", "--color=always"); err != nil {
		return err
	}
	if err := runGosec(); err != nil {
		return err
	}
	if err := runGitleaks(); err != nil {
		return err
	}
	if err := runArchitectureCheck(); err != nil {
		return err
	}
	if err := runComplexityCheck(); err != nil {
		return err
	}
	return runSLOCCheck()
}

func runDependencyCheck() error {
	if _, err := runCmd(5*time.Minute, "go", "mod", "tidy"); err != nil {
		return err
	}
	if _, err := runCmd(2*time.Minute, "go", "mod", "verify"); err != nil {
		return err
	}
	_, err := runCmd(1*time.Minute, "git", "diff", "--exit-code", "go.mod", "go.sum")
	return err
}

func runFormatCheck() error {
	args := append([]string{"-l"}, codeTargets...)
	out, err := runCmd(1*time.Minute, "gofmt", args...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}
	return errors.New("format check failed, unformatted files listed above")
}

func runGosec() error {
	if _, err := runCmd(15*time.Minute, "go", "run", "github.com/securego/gosec/v2/cmd/gosec@latest", "-fmt", "sarif", "-out", "gosec.sarif", "./..."); err != nil {
		return err
	}
	_, err := runCmd(15*time.Minute, "go", "run", "github.com/securego/gosec/v2/cmd/gosec@latest", "-severity", "medium", "./...")
	return err
}

func runGitleaks() error {
	variants := [][]string{
		{"run", "github.com/gitleaks/gitleaks/v8@latest", "git", ".", "--config=gitleaks.toml"},
		{"run", "github.com/gitleaks/gitleaks/v8@latest", "detect", "--source=.", "--config=gitleaks.toml"},
	}
	var lastErr error
	for _, args := range variants {
		_, err := runCmd(15*time.Minute, "go", args...)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("gitleaks check failed: %w", lastErr)
}

func runArchitectureCheck() error {
	out, err := runCmd(2*time.Minute, "go", "run", "scripts/arch_check.go")
	if writeErr := writeText("arch_report.txt", out); writeErr != nil {
		return writeErr
	}
	return err
}

func runComplexityCheck() error {
	args := []string{"run", "github.com/fzipp/gocyclo/cmd/gocyclo@latest", "-over", strconv.Itoa(cycloThreshold)}
	args = append(args, codeTargets...)
	out, err := runCmd(10*time.Minute, "go", args...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return errors.New("complexity threshold exceeded")
	}
	return nil
}
