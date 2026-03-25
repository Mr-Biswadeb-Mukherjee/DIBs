package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runSLOCCheck() error {
	files, err := collectGoFiles(codeTargets)
	if err != nil {
		return err
	}

	failed := false
	for _, file := range files {
		count, err := countSLOC(file)
		if err != nil {
			return err
		}
		if count > slocFail {
			fmt.Printf("ERROR: file too large %s (%d)\n", file, count)
			failed = true
			continue
		}
		if count > slocWarn {
			fmt.Printf("WARN: file getting large %s (%d)\n", file, count)
		}
	}
	if failed {
		return errors.New("SLOC threshold exceeded")
	}
	return nil
}

func collectGoFiles(targets []string) ([]string, error) {
	files := map[string]struct{}{}
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() && strings.HasSuffix(target, ".go") {
			files[target] = struct{}{}
			continue
		}
		err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			files[path] = struct{}{}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	out := make([]string, 0, len(files))
	for file := range files {
		out = append(out, file)
	}
	sort.Strings(out)
	return out, nil
}

func countSLOC(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sloc := 0
	inBlock := false
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line, block := stripComments(scan.Text(), inBlock)
		inBlock = block
		if strings.TrimSpace(line) != "" {
			sloc++
		}
	}
	if err := scan.Err(); err != nil {
		return 0, err
	}
	return sloc, nil
}

func stripComments(line string, inBlock bool) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(line); {
		if inBlock {
			end := strings.Index(line[i:], "*/")
			if end < 0 {
				return out.String(), true
			}
			i += end + 2
			inBlock = false
			continue
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
			inBlock = true
			i += 2
			continue
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
			break
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String(), inBlock
}

func runCmd(timeout time.Duration, name string, args ...string) (string, error) {
	return runCmdWithEnv(timeout, nil, name, args...)
}

func runCmdWithEnv(timeout time.Duration, env []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return buf.String(), fmt.Errorf("%s timed out after %s", name, timeout)
		}
		return buf.String(), fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return buf.String(), nil
}

func writeText(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
