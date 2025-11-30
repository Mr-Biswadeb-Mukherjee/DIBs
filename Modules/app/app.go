package app

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	recon "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon"

	config "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/config"
	cooldown "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/cooldown"
	filewriter "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/filewriter"
	progressBar "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/progressBar"
	ratelimiter "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/ratelimiter"
	redis "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/redis"
	ui "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/ui"
	wpkg "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/worker"
)

func Run(parentCtx context.Context) error {

	// UI start animation
	animStop := make(chan struct{})
	go ui.Spinner(animStop, "Starting Infermal_v2 Engine")
	startTime := ui.StartBanner()

	// Load config
	cfg, err := config.LoadOrCreateConfig("Setting/setting.conf")
	if err != nil {
		close(animStop)
		return fmt.Errorf("error loading config: %w", err)
	}

	// Redis init
	if err := redis.Init(); err != nil {
		close(animStop)
		return fmt.Errorf("error initializing redis: %w", err)
	}
	rdb := redis.Client()

	// Rate limiter
	limit := cfg.RateLimit
	if limit <= 0 {
		limit = 999999999
	}
	ratelimiter.Init(rdb, time.Second, int64(limit))

	// Recon init
	re := recon.New(
		cfg.UpstreamDNS,
		cfg.BackupDNS,
		int(cfg.DNSRetries),
		int(cfg.DNSTimeoutMS),
	)

	// Domain generation
	allGenerated, err := recon.GenerateDomains("Input/Keywords.csv")
	if err != nil {
		close(animStop)
		return fmt.Errorf("error processing Keywords.csv: %w", err)
	}

	total := int64(len(allGenerated))
	if total == 0 {
		close(animStop)
		fmt.Println("no domains generated")
		return nil
	}

	// CSV writer
	fw, err := filewriter.SafeNewCSVWriter("Input/Domains.csv", filewriter.Overwrite)
	if err != nil {
		close(animStop)
		return fmt.Errorf("error opening csv writer: %w", err)
	}

	// Worker pool
	opts := &wpkg.RunOptions{
		Timeout:         time.Duration(cfg.TimeoutSeconds) * time.Second,
		MaxRetries:      cfg.MaxRetries,
		AutoScale:       cfg.AutoScale,
		MinWorkers:      1,
		NonBlockingLogs: true,
	}

	wp := wpkg.NewWorkerPool(opts, runtime.NumCPU()*4, rdb)

	close(animStop)
	time.Sleep(150 * time.Millisecond)

	var completed int64
	var resolved int64

	cdm := cooldown.NewManager()
	cdm.StartWatcher()

	pb := progressBar.NewProgressBar(int(total), "Resolving domains", "green")
	pb.StartAutoRender(func() (int64, int64, bool, int64) {
		cur := atomic.LoadInt64(&completed)
		return cur, total, cdm.Active(), cdm.Remaining()
	})

	var wg sync.WaitGroup
	successTTL := 48 * time.Hour
	failTTL := 10 * time.Second

	// Worker tasks
	for _, domain := range allGenerated {
		d := domain

		taskFunc := func(ctx context.Context) (interface{}, []string, []string, []error) {

			// Redis cache
			cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			cached, err := rdb.GetValue(cacheCtx, "dns:"+d)
			cancel()

			if err == nil {
				if cached == "1" {
					return d, nil, nil, nil
				}
				if cached == "0" {
					return nil, nil, nil, nil
				}
			}

			// Cooldown
			if cdm.Active() {
				select {
				case <-ctx.Done():
					return nil, nil, nil, []error{ctx.Err()}
				case <-cdm.Gate():
				}
			}

			// Rate limit
			for {
				select {
				case <-ctx.Done():
					return nil, nil, nil, []error{ctx.Err()}
				default:
				}
				allowed, err := ratelimiter.RateLimit(ctx, "dns-rate")
				if err != nil {
					break
				}
				if allowed {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			ok, _ := re.Resolve(ctx, d)

			if ok {
				rdb.SetValue(context.Background(), "dns:"+d, "1", successTTL)
				return d, nil, nil, nil
			}

			rdb.SetValue(context.Background(), "dns:"+d, "0", failTTL)
			return nil, nil, nil, nil
		}

		_, resCh, err := wp.SubmitTask(taskFunc, wpkg.Medium, 0)
		if err != nil {
			atomic.AddInt64(&completed, 1)
			pb.Add(1)
			continue
		}

		wg.Add(1)
		go func(rc <-chan wpkg.WorkerResult) {
			defer wg.Done()

			res, ok := <-rc
			if !ok {
				atomic.AddInt64(&completed, 1)
				pb.Add(1)
				return
			}

			if s, ok := res.Result.(string); ok && s != "" {
				fw.WriteRow([]string{s})
				atomic.AddInt64(&resolved, 1)
			}

			newCount := atomic.AddInt64(&completed, 1)
			pb.Add(1)

			if cfg.CooldownAfter > 0 && newCount%int64(cfg.CooldownAfter) == 0 {
				cdm.Trigger(int64(cfg.CooldownDuration))
			}

		}(resCh)
	}

	wg.Wait()
	wp.Stop()
	fw.Close()

	pb.StopAutoRender()
	pb.Finish()

	ui.EndBanner(startTime, total, resolved)
	fmt.Println("✔ Valid domains written to Input/Domains.csv")

	redis.Close()

	return nil
}
