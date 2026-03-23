// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package engine

import (
	"context"
	"fmt"
)

var runApp = func(ctx context.Context, deps Dependencies) error {
	_ = normalizeContext(ctx)
	_ = deps
	return nil
}

var printLine = func(args ...any) {
	fmt.Println(args...)
}

func Run(deps Dependencies) {
	if err := runApp(context.Background(), deps); err != nil {
		printLine("Error:", err.Error())
	}

	printLine("Shutdown complete.")
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
