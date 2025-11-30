// ui_test.go

package ui

import (
	"fmt"
	"time"
)

func Spinner(stop chan struct{}, label string) {
	frames := []string{"|", "/", "-", "\\"}
	i := 0

	for {
		select {
		case <-stop:
			fmt.Printf("\r%-40s\n", label+" ✓")
			return
		default:
			fmt.Printf("\r%-40s", fmt.Sprintf("%s %s", label, frames[i%4]))
			i++
			time.Sleep(110 * time.Millisecond)
		}
	}
}

func StartBanner() time.Time {
	now := time.Now().Local()
	fmt.Printf("\n[%s] Engine Started\n", now.Format("02-01-06 03:04:05 PM"))
	return now
}

// EndBanner prints the stop timestamp and summary stats.
func EndBanner(start time.Time, total int64, resolved int64) {
	end := time.Now().Local()
	elapsed := time.Since(start).Truncate(time.Millisecond)

	fmt.Printf("\n[%s] Engine Finished\n", end.Format("02-01-06 03:04:05 PM"))
	fmt.Printf("✔ Resolution time: %s | Total checked: %d\n", elapsed, total)
	fmt.Printf("✔ Total resolved: %d\n", resolved)
}
