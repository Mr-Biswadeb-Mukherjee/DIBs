package adaptive

import (
	"math"
	"time"
)

func roundFloat(v float64) float64 {
	return math.Round(v)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a >= b {
		return a
	}
	return b
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}

func clampDuration(v, low, high time.Duration) time.Duration {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clampFloat(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clamp01(v float64) float64 {
	return clampFloat(v, 0, 1)
}

func clampInt64(v, low, high int64) int64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func safeRatio(n, d float64) float64 {
	if d <= 0 {
		return 0
	}
	return n / d
}

func ewma(prev, next, alpha float64) float64 {
	return prev*(1-alpha) + next*alpha
}

func durationSecondsCeil(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	secs := d / time.Second
	if d%time.Second == 0 {
		return int64(secs)
	}
	return int64(secs + 1)
}
