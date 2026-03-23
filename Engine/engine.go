// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package engine

import (
	"context"
	"fmt"
)

var runApp = runRuntime

var printLine = func(args ...any) {
	fmt.Println(args...)
}

func Run(deps Dependencies) {
	ctx := context.Background()

	if err := runApp(ctx, deps); err != nil {
		printLine("Error:", err.Error())
	}

	printLine("Shutdown complete.")
}

func runRuntime(parentCtx context.Context, deps Dependencies) error {
	rt, err := newAppRuntime(deps)
	if err != nil {
		return err
	}
	defer rt.Close()

	ctx := normalizeContext(parentCtx)
	domains, generatedMeta, err := loadGeneratedDomains(rt.paths.KeywordsCSV)
	if err != nil {
		rt.logs.app.Alert("domain generation failed: %v", err)
		return fmt.Errorf("error processing Keywords.csv: %w", err)
	}

	total := int64(len(domains))
	if total == 0 {
		rt.startup.Stop()
		printLine("no domains generated")
		return nil
	}

	runner := newScanRunner(rt, total)
	modules, err := rt.newModules(ctx, generatedMeta, runner.onIntelDone())
	if err != nil {
		rt.logs.app.Alert("intel pipeline init failed: %v", err)
		return fmt.Errorf("error starting dns intel pipeline: %w", err)
	}

	resolved := runner.run(ctx, domains, modules)
	rt.finishRun(total, resolved)
	rt.logs.app.Info("run completed generated=%d resolved=%d", total, resolved)
	return nil
}

func normalizeContext(parentCtx context.Context) context.Context {
	if parentCtx != nil {
		return parentCtx
	}
	return context.Background()
}
