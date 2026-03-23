// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	progressTick  = 300 * time.Millisecond
	progressWidth = 40

	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
)

type progressRow struct {
	label string
	color string
	get   func() (int64, int64, bool, int64)
	line  func(time.Time) string

	lastCount int64
	lastTime  time.Time
}

type liveProgress struct {
	rows []*progressRow
	stop chan struct{}

	once    sync.Once
	printed bool
	mu      sync.Mutex
}

func newLiveProgress(rows []*progressRow) *liveProgress {
	now := time.Now()
	for _, row := range rows {
		row.lastTime = now
	}
	return &liveProgress{
		rows: rows,
		stop: make(chan struct{}),
	}
}

func (lp *liveProgress) Start() {
	ticker := time.NewTicker(progressTick)
	go lp.loop(ticker)
}

func (lp *liveProgress) Stop() {
	lp.once.Do(func() { close(lp.stop) })
}

func (lp *liveProgress) Finish() {
	lp.render(time.Now())
	fmt.Print(colorReset)
}

func (lp *liveProgress) loop(ticker *time.Ticker) {
	defer ticker.Stop()
	for {
		select {
		case <-lp.stop:
			return
		case <-ticker.C:
			lp.render(time.Now())
		}
	}
}

func (lp *liveProgress) render(now time.Time) {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	lines := make([]string, 0, len(lp.rows))
	for _, row := range lp.rows {
		lines = append(lines, row.format(now))
	}
	lp.printLines(lines)
}

func (lp *liveProgress) printLines(lines []string) {
	if len(lines) == 0 {
		return
	}
	if lp.printed {
		fmt.Printf("\033[%dA", len(lines))
	}
	for _, line := range lines {
		fmt.Print("\r\033[2K")
		fmt.Println(line)
	}
	lp.printed = true
}

func (r *progressRow) format(now time.Time) string {
	if r.line != nil {
		return r.line(now)
	}
	cur, total, cooldownOn, cooldownRemaining := r.get()
	total = normalizeTotal(cur, total)
	rps := r.consumeRPS(now, cur)
	eta := estimateETA(cur, total, rps)
	pct := percent(cur, total)
	bar := buildProgressBar(cur, total, progressWidth)
	label := r.labelWithCooldown(cooldownOn, cooldownRemaining)
	return fmt.Sprintf("%s%s%s | %.2f req/s | ETA %s %3d%% %s",
		r.colorFor(cooldownOn), label, colorReset, rps, eta, pct, bar)
}

func (r *progressRow) consumeRPS(now time.Time, cur int64) float64 {
	elapsed := now.Sub(r.lastTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1e-6
	}
	delta := cur - r.lastCount
	if delta < 0 {
		delta = 0
	}
	r.lastCount = cur
	r.lastTime = now
	return float64(delta) / elapsed
}

func (r *progressRow) labelWithCooldown(active bool, remaining int64) string {
	if !active {
		return r.label
	}
	return fmt.Sprintf("%s (Cooldown %ds)", r.label, remaining)
}

func (r *progressRow) colorFor(cooldownOn bool) string {
	if cooldownOn {
		return colorYellow
	}
	return r.color
}

func normalizeTotal(cur, total int64) int64 {
	if total < 1 {
		total = 1
	}
	if cur > total {
		return cur
	}
	return total
}

func estimateETA(cur, total int64, rps float64) string {
	if rps <= 0 || cur >= total {
		return "—"
	}
	seconds := float64(total-cur) / rps
	d := time.Duration(seconds) * time.Second
	return d.Truncate(time.Second).String()
}

func percent(cur, total int64) int {
	if total <= 0 {
		return 0
	}
	p := int((cur * 100) / total)
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func buildProgressBar(cur, total int64, width int) string {
	if width < 1 {
		width = 1
	}
	filled := int((cur * int64(width)) / total)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return "|" + strings.Repeat("█", filled) + strings.Repeat(" ", width-filled) + "|"
}

func resolveProgressRow(total int64, completed *int64, cdm CooldownManager) *progressRow {
	return &progressRow{
		label: "Resolving domains",
		color: colorGreen,
		get: func() (int64, int64, bool, int64) {
			return atomic.LoadInt64(completed), total, cdm.Active(), cdm.Remaining()
		},
	}
}

func intelProgressRow(total int64, intelDone *int64) *progressRow {
	return &progressRow{
		label: "Extracting Intelligence",
		color: colorBlue,
		get: func() (int64, int64, bool, int64) {
			return atomic.LoadInt64(intelDone), total, false, 0
		},
	}
}

func generatedDomainsRow(total int64, resolved *int64) *progressRow {
	return &progressRow{
		line: func(time.Time) string {
			cur := atomic.LoadInt64(resolved)
			return fmt.Sprintf(
				"%sGenerated Domain:%s %d  | Resolved Domain: %d/%d",
				colorBlue, colorReset, total, cur, total,
			)
		},
	}
}
