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
	logger "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/logger"
	progressBar "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/progressBar"
	ratelimiter "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/ratelimiter"
	redis "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/redis"
	ui "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/ui"
	wpkg "github.com/official-biswadeb941/Infermal_v2/Modules/app/core/worker"
)

func Run(parentCtx context.Context) error {
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	appLog := logger.NewLogger("app")
	dnsLog := logger.NewLogger("dns")
	rateLimiterLog := logger.NewLogger("ratelimiter")
	defer appLog.Close()
	defer dnsLog.Close()
	defer rateLimiterLog.Close()

	const maxModuleErrorLogs = 25
	var moduleErrorCount int64
	logModuleError := func(module, scope string, err error) {
		if err == nil {
			return
		}
		if atomic.AddInt64(&moduleErrorCount, 1) > maxModuleErrorLogs {
			return
		}
		appLog.Warning("%s error scope=%s err=%v", module, scope, err)
	}

	// UI start animation
	animStop := make(chan struct{})
	go ui.Spinner(animStop, "Starting Infermal_v2 Engine")
	startTime := ui.StartBanner()

	// Load config
	cfg, err := config.LoadOrCreateConfig("Setting/setting.conf")
	if err != nil {
		close(animStop)
		appLog.Alert("config load failed: %v", err)
		return fmt.Errorf("error loading config: %w", err)
	}

	// Redis init
	if err := redis.Init(); err != nil {
		close(animStop)
		appLog.Alert("redis init failed: %v", err)
		return fmt.Errorf("error initializing redis: %w", err)
	}
	rdb := redis.Client()
	defer redis.Close()

	// Recon init
	re := recon.New(
		cfg.UpstreamDNS,
		cfg.BackupDNS,
		int(cfg.DNSRetries),
		int(cfg.DNSTimeoutMS),
		dnsLog,
	)

	// Domain generation
	allGenerated, err := recon.GenerateDomains("Input/Keywords.csv")
	if err != nil {
		close(animStop)
		appLog.Alert("domain generation failed: %v", err)
		return fmt.Errorf("error processing Keywords.csv: %w", err)
	}

	total := int64(len(allGenerated))
	if total == 0 {
		close(animStop)
		fmt.Println("no domains generated")
		return nil
	}

	workers := runtime.NumCPU() * 4
	initialRate := seedRateLimit(cfg.RateLimit, total, workers)
	tuner := newRuntimeTuner(cfg, total, workers)

	// CSV writer
	fw, err := filewriter.SafeNewCSVWriter("Input/Domains.csv", filewriter.Overwrite)
	if err != nil {
		close(animStop)
		appLog.Alert("csv writer init failed: %v", err)
		return fmt.Errorf("error opening csv writer: %w", err)
	}

	// Rate limiter
	ratelimiter.Init(rdb, time.Second, initialRate, rateLimiterLog)

	workerTimeout := tuner.resolveTimeout() * 4
	if workerTimeout < 12*time.Second {
		workerTimeout = 12 * time.Second
	}

	var activeWorkers int64
	opts := &wpkg.RunOptions{
		Timeout:         workerTimeout,
		MaxRetries:      cfg.MaxRetries,
		AutoScale:       cfg.AutoScale,
		MinWorkers:      1,
		NonBlockingLogs: true,
		OnTaskStart: func(taskID int64) {
			_ = taskID
			atomic.AddInt64(&activeWorkers, 1)
		},
		OnTaskFinish: func(taskID int64, res wpkg.WorkerResult) {
			_ = taskID
			_ = res
			atomic.AddInt64(&activeWorkers, -1)
		},
	}

	wp := wpkg.NewWorkerPool(opts, workers, rdb)

	close(animStop)
	time.Sleep(150 * time.Millisecond)

	var submitted int64
	var completed int64
	var resolved int64

	cdm := cooldown.NewManager()
	cdm.StartWatcher()
	controlCtx, controlCancel := context.WithCancel(parentCtx)
	defer controlCancel()
	go tuner.run(controlCtx, cdm, runtimeCounters{
		submitted: &submitted,
		completed: &completed,
		active:    &activeWorkers,
	}, appLog)

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

		taskFunc := makeDomainTask(d, re, rdb, cdm, tuner, successTTL, failTTL, logModuleError)

		_, resCh, err := wp.SubmitTask(taskFunc, wpkg.Medium, 0)
		if err != nil {
			logModuleError("worker-submit", d, err)
			atomic.AddInt64(&completed, 1)
			pb.Add(1)
			continue
		}
		atomic.AddInt64(&submitted, 1)

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
			for _, taskErr := range res.Errors {
				logModuleError("worker-task", "result", taskErr)
			}

			atomic.AddInt64(&completed, 1)
			pb.Add(1)

		}(resCh)
	}

	wg.Wait()
	wp.Stop()
	fw.Close()

	pb.StopAutoRender()
	pb.Finish()

	ui.EndBanner(startTime, total, resolved)
	fmt.Println("✔ Valid domains written to Input/Domains.csv")
	appLog.Info("run completed generated=%d resolved=%d", total, resolved)

	return nil
}
