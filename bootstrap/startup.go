// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"sync"
	"time"

	ui "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/ui"
)

type startupAdapter struct {
	label    string
	animStop chan struct{}
	stopOnce sync.Once
}

func newStartupAdapter() *startupAdapter {
	return &startupAdapter{}
}

func (s *startupAdapter) Start(label string) time.Time {
	s.label = label
	s.animStop = make(chan struct{})
	go ui.Spinner(s.animStop, label)
	return ui.StartBanner()
}

func (s *startupAdapter) Stop() {
	if s == nil || s.animStop == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.animStop)
		time.Sleep(150 * time.Millisecond)
	})
}

func (s *startupAdapter) Finish(start time.Time, total, resolved int64) {
	ui.EndBanner(start, total, resolved)
}
