// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package engine

import (
	"context"
	"fmt"

	app "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app"
)

var runApp = app.Run

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
