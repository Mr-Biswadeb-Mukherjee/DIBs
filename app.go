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
	ratelimiter "github.com/official-biswadeb941/Infermal_v2/Modules/Ratelimiter"
)

//
// ------------------------------
//  Redis Interface (Only Here)
// ------------------------------
//
type RedisStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
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

	// Start timestamp (system time, 12-hour format, dd-mm-yy)
	startTime := time.Now().Local()
	fmt.Printf("\n[%s] Engine Started\n",
		startTime.Format("02-01-06 03:04:05 PM"))

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

	var rdb RedisStore = redis.Client()

	// Initialize sliding-window ratelimiter (uses the same Redis backend)
	// If cfg.RateLimit <= 0 treat as effectively unlimited by using a very large number.
	limit := cfg.RateLimit
	if limit <= 0 {
		limit = 999999999
	}

	ratelimiter.Init(
		rdb,                  // redis store (must implement Eval)
		time.Second,          // sliding window duration (1 second window)
		int64(limit),         // max hits per window
	)

	dns := dnsengine.New(dnsengine.Config{
		Upstream:  cfg.UpstreamDNS,
		Backup:    cfg.BackupDNS,
		Retries:   cfg.DNSRetries,
		TimeoutMS: cfg.DNSTimeoutMS,
	})

	keywords, err := domain_generator.LoadKeywordsCSV("Input/Keywords.csv")
	if err != nil {
		close(animStop)
		fmt.Fprintf(os.Stderr, "\nError loading Keywords.csv: %v\n", err)
		os.Exit(1)
	}

	var allGenerated []string
	for _, base := range keywords {
		groups := domain_generator.DomainGenerator(base)
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

	fw, err := filewriter.SafeNewCSVWriter("Input/Domains.csv", filewriter.Overwrite)
	if err != nil {
		close(animStop)
		fmt.Println("Error opening CSV writer:", err)
		os.Exit(1)
	}

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

	for _, domain := range allGenerated {
		d := domain

		taskFunc := func(ctx context.Context) (interface{}, []string, []string, []error) {

			// Check Redis cache first
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

			// Cooldown gate
			if cdm.Active() {
				select {
				case <-ctx.Done():
					return nil, nil, nil, []error{ctx.Err()}
				case <-cdm.Gate():
				}
			}

			// ------------------------
			// NEW Redis Rate Limiter
			// ------------------------
			// Use a per-key limiter name; using "dns-rate" for all DNS calls.
			// We will fail-open on Eval errors (so transient redis problems don't stall everything).
			for {
				select {
				case <-ctx.Done():
					return nil, nil, nil, []error{ctx.Err()}
				default:
				}

				allowed, err := ratelimiter.RateLimit(ctx, "dns-rate")
				if err != nil {
					// Fail-open: if limiter errors (redis unavailable, etc.), break and continue.
					// This avoids blocking resolution due to transient limiter failures.
					break
				}
				if allowed {
					break
				}
				// Not allowed yet — back off briefly then retry.
				// Short sleep keeps latency low while not burning CPU.
				time.Sleep(10 * time.Millisecond)
			}

			// DNS resolve
			ok, _ := dns.Resolve(ctx, d)

			if ok {
				if rdb != nil {
					go func(domain string) {
						cctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
						_ = rdb.SetValue(cctx, "dns:"+domain, "1", successTTL)
						cancel()
					}(d)
				}
				return d, nil, nil, nil
			}

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

	_ = fw.Close()

	pb.StopAutoRender()
	pb.Finish()

	elapsed := time.Since(start).Truncate(time.Millisecond)

	// End timestamp (system time, 12-hour format, dd-mm-yy)
	endTime := time.Now().Local()
	fmt.Printf("\n[%s] Engine Finished\n",
		endTime.Format("02-01-06 03:04:05 PM"))

	fmt.Printf("✔ Resolution complete. Time: %s | Total checked: %d\n", elapsed, total)
	fmt.Printf("✔ Total Resolved Domains: %d\n", resolved)
	fmt.Println("✔ Valid domains written to Input/Domains.csv")

	redis.Close()
}
