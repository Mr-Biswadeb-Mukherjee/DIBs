// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package main

import (
	"context"
	"fmt"

	app "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app"
	bootstrap "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/bootstrap"
)

func main() {
	deps, err := bootstrap.BuildEngineDependencies()
	if err != nil {
		fmt.Println("Error:", err.Error())
		return
	}
	if err := app.Run(context.Background(), deps); err != nil {
		fmt.Println("Error:", err.Error())
	}
	fmt.Println("Shutdown complete.")
}
