package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	config "github.com/official-biswadeb941/Infermal_v2/Modules/Config"
	domain_generator "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator"
	dnsengine "github.com/official-biswadeb941/Infermal_v2/Modules/DNS"
	progressBar "github.com/official-biswadeb941/Infermal_v2/Modules/Progressbar"
	wpkg "github.com/official-biswadeb941/Infermal_v2/Modules/Worker"
	cooldown "github.com/official-biswadeb941/Infermal_v2/Modules/Cooldown"
	filewriter "github.com/official-biswadeb941/Infermal_v2/Modules/Filewriter"
	redis "github.com/official-biswadeb941/Infermal_v2/Modules/Redis"
)

//
// ------------------------------
//  Redis Interface (Only Here)
// ------------------------------
//
type RedisStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
}

func startAnimation(stopChan chan struct{}) {
	frames := []string{"|", "/", "-", "\\"}
	i := 0
	base := "Starting Infermal_v2 Engine"

	for {
		select {
		case <-stopChan:
			fmt.Printf("\r%-40s\n", base+" ✓")
			return
		default:
			frame := frames[i%len(frames)]
			fmt.Printf("\r%-40s", fmt.Sprintf("%s %s", base, frame))
			i++
			time.Sleep(110 * time.Millisecond)
		}
	}
}

func main() {

	animStop := make(chan struct{})
	go startAnimation(animStop)

	// Load config
	cfg, err := config.LoadOrCreateConfig("Setting/setting.conf")
	if err != nil {
		close(animStop)
		fmt.Println("\nError loading config:", err)
		os.Exit(1)
	}

	// Init Redis
	if err := redis.Init(); err != nil {
		close(animStop)
		fmt.Println("\nError initializing Redis:", err)
		os.Exit(1)
	}

	// Wrap Redis client behind interface
	var rdb RedisStore = redis.Client()

	// DNS engine
	dns := dnsengine.New(dnsengine.Config{
		Upstream:  cfg.UpstreamDNS,
		Backup:    cfg.BackupDNS,
		Retries:   cfg.DNSRetries,
		TimeoutMS: cfg.DNSTimeoutMS,
	})

	// Load keywords
	keywords, err := domain_generator.LoadKeywordsCSV("Input/Keywords.csv")
	if err != nil {
		close(animStop)
		fmt.Fprintf(os.Stderr, "\nError loading Keywords.csv: %v\n", err)
		os.Exit(1)
	}

	// Generate domains
	var allGenerated []string
	for _, base := range keywords {
		groups := domain_generator.RunAll(base)
		for _, g := range groups {
			allGenerated = append(allGenerated, g...)
		}
	}

	total := int64(len(allGenerated))
	if total == 0 {
		close(animStop)
		fmt.Println("No domains generated. Exiting.")
		return
	}

	// Async CSV writer
	fw, err := filewriter.SafeNewCSVWriter("Input/Malicious_Domains.csv", filewriter.Overwrite)
	if err != nil {
		close(animStop)
		fmt.Println("Error opening CSV writer:", err)
		os.Exit(1)
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

	var completed int64 = 0
	var resolved int64 = 0
	start := time.Now()

	// RATE LIMITER: guard against zero and give a stoppable ticker
	var rateTicker *time.Ticker
	if cfg.RateLimit <= 0 {
		// default to 1 req/s if misconfigured
		rateTicker = time.NewTicker(time.Second)
	} else {
		interval := time.Second / time.Duration(cfg.RateLimit)
		if interval <= 0 {
			interval = time.Second
		}
		rateTicker = time.NewTicker(interval)
	}
	defer rateTicker.Stop()

	// Cooldown manager
	cdm := cooldown.NewManager()
	cdm.StartWatcher()

	// Progress bar
	pb := progressBar.NewProgressBar(int(total), "Resolving domains", "green")
	pb.StartAutoRender(func() (int64, int64, bool, int64) {
		cur := atomic.LoadInt64(&completed)
		return cur, total, cdm.Active(), cdm.Remaining()
	})

	var wg sync.WaitGroup

	// -------------------------
	// TTL for Redis cache (FIX)
	// -------------------------
	successTTL := 48 * time.Hour // success stays cached long
	failTTL := 10 * time.Second  // failure cache expires fast

	// Dispatch jobs
	for _, domain := range allGenerated {
		d := domain

		taskFunc := func(ctx context.Context) (interface{}, []string, []string, []error) {

			// Redis lookup BEFORE DNS (use short context so Redis can't block forever)
			if rdb != nil {
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
			}

			// cooldown + rate-limit (context-aware)
			if cdm.Active() {
				select {
				case <-ctx.Done():
					return nil, nil, nil, []error{ctx.Err()}
				case <-cdm.Gate():
				}
			}

			select {
			case <-ctx.Done():
				return nil, nil, nil, []error{ctx.Err()}
			case <-rateTicker.C:
			}

			ok, _ := dns.Resolve(ctx, d)

			if ok {
				// async best-effort set (short timeout)
				if rdb != nil {
					go func(domain string) {
						cctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
						_ = rdb.SetValue(cctx, "dns:"+domain, "1", successTTL)
						cancel()
					}(d)
				}
				return d, nil, nil, nil
			}

			// fail TTL drastically reduced — write best-effort
			if rdb != nil {
				go func(domain string) {
					cctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
					_ = rdb.SetValue(cctx, "dns:"+domain, "0", failTTL)
					cancel()
				}(d)
			}

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
				// write to CSV (filewriter handles its own sync)
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

	// Wait
	wg.Wait()
	wp.Stop()

	// close CSV and progressbar
	_ = fw.Close()

	pb.StopAutoRender()
	pb.Finish()

	elapsed := time.Since(start).Truncate(time.Millisecond)

	fmt.Printf("\n✔ Resolution complete. Time: %s | Total checked: %d\n", elapsed, total)
	fmt.Printf("✔ Total Resolved Domains: %d\n", resolved)
	fmt.Println("✔ Valid domains written to Input/Malicious_Domains.csv")

	redis.Close()
}
