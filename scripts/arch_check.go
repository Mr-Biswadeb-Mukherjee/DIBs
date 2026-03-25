package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Package struct {
	ImportPath string   `json:"ImportPath"`
	Dir        string   `json:"Dir"`
	Imports    []string `json:"Imports"`
}

const (
	penaltyCross      = 5
	penaltyHorizontal = 3
	penaltyRule3      = 2
	maxPenaltyCap     = 30.0
)

func runGoList() ([]Package, error) {
	cmd := exec.Command("go", "-C", "Engine", "list", "-json", "./...")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []Package

	for {
		var p Package
		if err := dec.Decode(&p); err != nil {
			break
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// Normalize to repo-relative path
func normalizePath(dir string) string {
	cwd, _ := os.Getwd()
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		return dir
	}
	return filepath.ToSlash(rel)
}

// Layer = first folder
func getLayer(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Module = second folder (if exists)
func getModule(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Parent = same directory
func getParent(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return path
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

type analysisContext struct {
	graph   map[string][]string
	pathMap map[string]string
	usage   map[string]int
}

func main() {
	pkgs, err := runGoList()
	if err != nil {
		fmt.Println("❌ go list failed:", err)
		os.Exit(0)
	}
	ctx := buildAnalysisContext(pkgs)
	violations, score := analyzeGraph(ctx)
	penalty := printReport(violations, score)
	_ = writePenaltyFile("arch_score.txt", penalty)
	os.Exit(0)
}

func buildAnalysisContext(pkgs []Package) analysisContext {
	ctx := analysisContext{
		graph:   make(map[string][]string),
		pathMap: make(map[string]string),
		usage:   make(map[string]int),
	}
	for _, p := range pkgs {
		ctx.pathMap[p.ImportPath] = normalizePath(p.Dir)
		ctx.graph[p.ImportPath] = p.Imports
		for _, imp := range p.Imports {
			ctx.usage[imp]++
		}
	}
	return ctx
}

func analyzeGraph(ctx analysisContext) ([]string, int) {
	var violations []string
	score := 0
	for pkg, imports := range ctx.graph {
		pathA := ctx.pathMap[pkg]
		for _, imp := range imports {
			pathB, ok := ctx.pathMap[imp]
			if !ok {
				continue
			}
			v, delta := evaluateRules(pathA, pathB, imp, ctx.usage)
			violations = append(violations, v...)
			score += delta
		}
	}
	return violations, score
}

func evaluateRules(pathA, pathB, imp string, usage map[string]int) ([]string, int) {
	violations := make([]string, 0, 3)
	score := 0
	if isCrossModule(pathA, pathB) {
		violations = append(violations, fmt.Sprintf("[CROSS] %s → %s", pathA, pathB))
		score += penaltyCross
	}
	if !isHorizontal(pathA, pathB) {
		return violations, score
	}
	violations = append(violations, fmt.Sprintf("[HORIZONTAL] %s → %s", pathA, pathB))
	score += penaltyHorizontal
	if usage[imp] > 1 {
		violations = append(violations, fmt.Sprintf("[RULE3] %s used by %d packages", pathB, usage[imp]))
		score += penaltyRule3
	}
	return violations, score
}

func isCrossModule(pathA, pathB string) bool {
	layerA, layerB := getLayer(pathA), getLayer(pathB)
	moduleA, moduleB := getModule(pathA), getModule(pathB)
	return layerA == layerB && moduleA != "" && moduleB != "" && moduleA != moduleB
}

func isHorizontal(pathA, pathB string) bool {
	return getParent(pathA) == getParent(pathB) && pathA != pathB
}

func printReport(violations []string, score int) float64 {
	fmt.Println("\n🏛️  Architectural Analysis Report")
	fmt.Println("────────────────────────────────────────")
	if len(violations) == 0 {
		fmt.Println("✅ No architectural violations detected.")
	} else {
		for _, v := range violations {
			fmt.Println("❌", v)
		}
	}
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("📉 Violation Score: %d\n", score)
	penalty := float64(score) * 0.5
	if penalty > maxPenaltyCap {
		penalty = maxPenaltyCap
	}
	fmt.Printf("📊 Suggested Coverage Penalty: -%.1f%%\n", penalty)
	return penalty
}

func writePenaltyFile(path string, penalty float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%.2f", penalty)
	return err
}
