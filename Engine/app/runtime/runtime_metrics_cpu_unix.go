// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

//go:build !windows

package runtime

import "syscall"

func processCPUSeconds() float64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	user := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1_000_000
	sys := float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1_000_000
	return user + sys
}
