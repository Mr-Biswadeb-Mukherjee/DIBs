package main

import (
	"context"

	app "github.com/official-biswadeb941/Infermal_v2/Modules/app"
)

func main() {
	// No signal handling — pure context
	ctx := context.Background()

	if err := app.Run(ctx); err != nil {
		println("Error:", err.Error())
	}

	println("Shutdown complete.")
}
