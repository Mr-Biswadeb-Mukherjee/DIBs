// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package main

import (
	"fmt"

	engine "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine"
	bootstrap "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/bootstrap"
)

func main() {
	deps, err := bootstrap.BuildEngineDependencies()
	if err != nil {
		fmt.Println("Error:", err.Error())
		return
	}
	engine.Run(deps)
}
