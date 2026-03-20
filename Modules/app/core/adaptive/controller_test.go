package adaptive

import (
	"testing"
	"time"
)

func TestEvaluateLowPressureIncreasesRate(t *testing.T) {
	cfg := DefaultConfig(120, 3*time.Second, 8)
	cfg.CooldownMinGap = 2 * time.Second

	c := NewController(cfg)
	for i := 0; i < 40; i++ {
		c.ObserveTask(180*time.Millisecond, false)
	}

	decision := c.Evaluate(Snapshot{
		QueueDepth:     0,
		InFlight:       2,
		ActiveWorkers:  8,
		CompletedDelta: 40,
	})

	if decision.RateLimit <= cfg.InitialRate {
		t.Fatalf("expected rate to increase, got initial=%d new=%d", cfg.InitialRate, decision.RateLimit)
	}
	if decision.Timeout > cfg.InitialTimeout {
		t.Fatalf("timeout should not increase on healthy path, initial=%v new=%v", cfg.InitialTimeout, decision.Timeout)
	}
	if decision.Cooldown != 0 {
		t.Fatalf("cooldown must stay disabled for healthy pressure, got=%v", decision.Cooldown)
	}
}

func TestEvaluateHighPressureCutsRateAndTriggersCooldown(t *testing.T) {
	cfg := DefaultConfig(220, 2*time.Second, 6)
	cfg.CooldownMinGap = 1 * time.Second
	cfg.CooldownMin = 2 * time.Second
	cfg.CooldownMax = 8 * time.Second

	c := NewController(cfg)
	for i := 0; i < 50; i++ {
		c.ObserveTask(6*time.Second, true)
		c.ObserveRateLimited(9)
	}
	for i := 0; i < 8; i++ {
		c.ObserveLimiterError()
	}

	decision := c.Evaluate(Snapshot{
		QueueDepth:     600,
		InFlight:       280,
		ActiveWorkers:  6,
		CompletedDelta: 0,
	})

	if decision.RateLimit >= cfg.InitialRate {
		t.Fatalf("expected rate reduction, initial=%d new=%d", cfg.InitialRate, decision.RateLimit)
	}
	if decision.Timeout <= cfg.InitialTimeout {
		t.Fatalf("timeout should expand on high pressure, initial=%v new=%v", cfg.InitialTimeout, decision.Timeout)
	}
	if decision.Cooldown == 0 {
		t.Fatalf("expected cooldown trigger under sustained pressure")
	}
}

func TestCooldownRespectsMinGap(t *testing.T) {
	cfg := DefaultConfig(180, 2*time.Second, 4)
	cfg.CooldownMinGap = 2 * time.Second

	c := NewController(cfg)
	for i := 0; i < 30; i++ {
		c.ObserveTask(8*time.Second, true)
		c.ObserveRateLimited(10)
	}

	first := c.Evaluate(Snapshot{
		QueueDepth:     700,
		InFlight:       300,
		ActiveWorkers:  4,
		CompletedDelta: 0,
	})
	if first.Cooldown == 0 {
		t.Fatalf("expected first cooldown trigger")
	}

	for i := 0; i < 20; i++ {
		c.ObserveTask(7*time.Second, true)
		c.ObserveRateLimited(8)
	}

	second := c.Evaluate(Snapshot{
		QueueDepth:     500,
		InFlight:       200,
		ActiveWorkers:  4,
		CompletedDelta: 0,
	})
	if second.Cooldown != 0 {
		t.Fatalf("cooldown should be suppressed during min gap, got=%v", second.Cooldown)
	}
}

func TestDefaultConfigBuildsSafeBounds(t *testing.T) {
	cfg := DefaultConfig(0, 0, 0)

	if cfg.InitialRate <= 0 {
		t.Fatalf("initial rate must be positive")
	}
	if cfg.MinRate <= 0 || cfg.MaxRate < cfg.InitialRate {
		t.Fatalf("invalid rate bounds: min=%d initial=%d max=%d", cfg.MinRate, cfg.InitialRate, cfg.MaxRate)
	}
	if cfg.MinTimeout <= 0 || cfg.MaxTimeout < cfg.InitialTimeout {
		t.Fatalf("invalid timeout bounds: min=%v initial=%v max=%v", cfg.MinTimeout, cfg.InitialTimeout, cfg.MaxTimeout)
	}
	if cfg.EvalInterval <= 0 || cfg.TargetLatency <= 0 {
		t.Fatalf("invalid runtime timing defaults")
	}
}
