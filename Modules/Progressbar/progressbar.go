package progressBar

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorGreen  = "\033[32m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
)

// ----------------------------------------------
// ProgressBar STRUCT
// ----------------------------------------------

type ProgressBar struct {
	bar       *progressbar.ProgressBar
	lock      sync.Mutex
	colorCode string

	// Stats for RPS + ETA
	lastCount int64
	lastTime  time.Time

	// Render loop
	stopRender chan struct{}
}

// ----------------------------------------------
// Constructor
// ----------------------------------------------

func NewProgressBar(total int, description string, color string) *ProgressBar {
	colorCode := ColorGreen
	switch color {
	case "red":
		colorCode = ColorRed
	case "yellow":
		colorCode = ColorYellow
	case "blue":
		colorCode = ColorBlue
	case "green":
		colorCode = ColorGreen
	}

	theme := progressbar.Theme{
		Saucer:        "█",
		SaucerHead:    "█",
		SaucerPadding: " ",
		BarStart:      "|",
		BarEnd:        "|",
	}

	bar := progressbar.NewOptions(
		total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetTheme(theme),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Print(ColorReset)
		}),
	)

	return &ProgressBar{
		bar:        bar,
		colorCode:  colorCode,
		lastTime:   time.Now(),
		stopRender: make(chan struct{}),
	}
}

// ----------------------------------------------
// Core Update Logic (called by app.go)
// ----------------------------------------------

func (p *ProgressBar) Update(current, total int64, cooldown bool, cooldownRemaining int64) {
	p.lock.Lock()
	defer p.lock.Unlock()

	now := time.Now()
	elapsed := now.Sub(p.lastTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1e-6
	}

	delta := float64(current - p.lastCount)
	rps := delta / elapsed

	var etaStr = "—"
	if rps > 0 && current < total {
		remaining := float64(total-current) / rps
		eta := time.Duration(remaining) * time.Second
		etaStr = eta.Truncate(time.Second).String()
	}

	// cooldown display
	desc := fmt.Sprintf(
		"%sResolving domains%s | %.2f req/s | ETA %s",
		p.colorForCooldown(cooldown),
		ColorReset,
		rps,
		etaStr,
	)

	if cooldown {
		desc = fmt.Sprintf(
			"%sResolving domains (Cooldown %ds)%s | %.2f req/s | ETA %s",
			ColorYellow,
			cooldownRemaining,
			ColorReset,
			rps,
			etaStr,
		)
	}

	p.bar.Describe(desc)
	_ = p.bar.RenderBlank()

	// update stats
	p.lastCount = current
	p.lastTime = now
}

func (p *ProgressBar) colorForCooldown(cooldown bool) string {
	if cooldown {
		return ColorYellow
	}
	return p.colorCode
}

// ----------------------------------------------
// Auto Render (ticker powered)
// ----------------------------------------------

func (p *ProgressBar) StartAutoRender(
	get func() (current int64, total int64, cooldown bool, remaining int64),
) {

	ticker := time.NewTicker(300 * time.Millisecond)

	go func() {
		for {
			select {
			case <-p.stopRender:
				ticker.Stop()
				return

			case <-ticker.C:
				cur, total, cd, rem := get()
				p.Update(cur, total, cd, rem)
			}
		}
	}()
}

func (p *ProgressBar) StopAutoRender() {
	close(p.stopRender)
}

// ----------------------------------------------
// Manual Methods
// ----------------------------------------------

func (p *ProgressBar) Add(n int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	_ = p.bar.Add(n)
}

func (p *ProgressBar) Render() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.bar.RenderBlank()
}

func (p *ProgressBar) Finish() {
	p.lock.Lock()
	defer p.lock.Unlock()
	_ = p.bar.Finish()
	fmt.Print(ColorReset)
}
