package adaptive

import "time"

func (c *Controller) updateAveragesLocked(w sampleWindow) {
	taskCount := maxInt64(w.tasks, 1)
	avgLatency := float64(c.cfg.TargetLatency)
	if w.tasks > 0 {
		avgLatency = float64(w.latencyNanos) / float64(taskCount)
	}

	errRate := float64(w.pressureErrs) / float64(taskCount)
	rateWait := float64(w.rateLimited) / float64(taskCount)
	limiterRate := float64(w.limiterErrs) / float64(taskCount)

	c.latencyEWMA = ewma(c.latencyEWMA, avgLatency, 0.22)
	c.errorEWMA = ewma(c.errorEWMA, errRate, 0.34)
	c.rateWaitEWMA = ewma(c.rateWaitEWMA, rateWait, 0.26)
	c.limiterErrEWMA = ewma(c.limiterErrEWMA, limiterRate, 0.30)
}

func (c *Controller) updateStallLocked(s Snapshot) {
	workers := maxInt64(s.ActiveWorkers, 1)
	stalled := s.InFlight > workers*3 && s.CompletedDelta == 0
	if stalled {
		c.stallTicks++
		return
	}
	if c.stallTicks > 0 {
		c.stallTicks--
	}
}

func (c *Controller) pressureLocked(s Snapshot) float64 {
	workers := float64(maxInt64(s.ActiveWorkers, 1))
	queuePerWorker := safeRatio(float64(s.QueueDepth), workers)
	inFlightPerWorker := safeRatio(float64(s.InFlight), workers)

	queueLoad := clampFloat(queuePerWorker/2.0, 0, 1)
	inflightLoad := clampFloat(inFlightPerWorker/3.0, 0, 1)
	latencyLoad := clampFloat(c.latencyEWMA/float64(c.cfg.TargetLatency), 0, 1.4)
	errorLoad := clampFloat(c.errorEWMA*3.0, 0, 1)
	rateLoad := clampFloat(c.rateWaitEWMA/8.0, 0, 1)
	limiterLoad := clampFloat(c.limiterErrEWMA*4.0, 0, 1)

	pressure := queueLoad*0.29 + inflightLoad*0.16 + latencyLoad*0.21
	pressure += errorLoad*0.19 + rateLoad*0.10 + limiterLoad*0.05
	if c.stallTicks >= 2 {
		pressure += 0.15
	}

	return clampFloat(pressure, 0, 1)
}

func (c *Controller) tuneRateLocked(pressure float64) int64 {
	next := c.rateLimit
	switch {
	case pressure >= 0.80:
		next = int64(float64(next) * 0.72)
	case pressure <= 0.33:
		next += maxInt64(1, next/9)
	case pressure >= 0.62:
		next = int64(float64(next) * 0.92)
	}

	c.rateLimit = clampInt64(next, c.cfg.MinRate, c.cfg.MaxRate)
	return c.rateLimit
}

func (c *Controller) tuneTimeoutLocked(pressure float64) time.Duration {
	target := time.Duration(c.latencyEWMA * 2.9)
	target = clampDuration(target, c.cfg.MinTimeout, c.cfg.MaxTimeout)
	if pressure > 0.80 {
		target += 300 * time.Millisecond
	}
	if pressure < 0.30 {
		target -= 150 * time.Millisecond
	}
	target = clampDuration(target, c.cfg.MinTimeout, c.cfg.MaxTimeout)

	if target > c.timeout {
		step := minDuration(500*time.Millisecond, target-c.timeout)
		c.timeout += step
	} else if target < c.timeout {
		step := minDuration(350*time.Millisecond, c.timeout-target)
		c.timeout -= step
	}

	c.timeout = clampDuration(c.timeout, c.cfg.MinTimeout, c.cfg.MaxTimeout)
	return c.timeout
}

func (c *Controller) cooldownLocked(pressure float64) time.Duration {
	if pressure < 0.93 && c.stallTicks < c.cfg.StallTickTrigger {
		return 0
	}
	if time.Since(c.lastCooldown) < c.cfg.CooldownMinGap {
		return 0
	}

	c.lastCooldown = time.Now()
	if c.stallTicks >= c.cfg.StallTickTrigger {
		return c.cfg.CooldownMax
	}
	return cooldownDuration(pressure, c.cfg.CooldownMin, c.cfg.CooldownMax)
}

func cooldownDuration(pressure float64, minDur, maxDur time.Duration) time.Duration {
	if maxDur <= minDur {
		return minDur
	}
	p := clampFloat((pressure-0.93)/0.07, 0, 1)
	span := float64((maxDur - minDur).Nanoseconds())
	inc := int64(roundFloat(span * p))
	return minDur + time.Duration(inc)
}
