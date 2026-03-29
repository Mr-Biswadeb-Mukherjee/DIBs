// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

//go:build windows

package runtime

import "syscall"

const windowsFiletimeTicksPerSecond = 10_000_000.0

func filetimeSeconds(ft syscall.Filetime) float64 {
	ticks := (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
	return float64(ticks) / windowsFiletimeTicksPerSecond
}

// processCPUSeconds returns process user+kernel CPU seconds on Windows.
func processCPUSeconds() float64 {
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0
	}
	var created syscall.Filetime
	var exited syscall.Filetime
	var kernel syscall.Filetime
	var user syscall.Filetime
	if err := syscall.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0
	}
	return filetimeSeconds(user) + filetimeSeconds(kernel)
}
