package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	config "github.com/official-biswadeb941/Infermal_v2/Modules/Config"
	domain_generator "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator"
	DNS "github.com/official-biswadeb941/Infermal_v2/Modules/DNS"
	wpkg "github.com/official-biswadeb941/Infermal_v2/Modules/Worker"
	progressBar "github.com/official-biswadeb941/Infermal_v2/Modules/Progressbar"
)

// GLOBAL cooldown state (atomic)
var cooldownActive int32 = 0
var cooldownUntil int64 = 0

// used to block worker goroutines during cooldown
var cooldownGate = make(chan struct{})

// Startup animation (unchanged)
func startAnimation(stopChan chan struct{}) {
	frames := []string{"|", "/", "-", "\\"}
	i := 0

	for {
		select {
		case <-stopChan:
			fmt.Print("\rStarting Infermal_v2 Engine ✓           \n")
			return
		default:
			fmt.Printf("\rStarting Infermal_v2 Engine %s", frames[i%len(frames)])
			i++
			time.Sleep(120 * time.Millisecond)
		}
	}
}

// small helper to trigger cooldown (recreates gate so workers block)
func triggerCooldown(durSeconds int64) {
	atomic.StoreInt32(&cooldownActive, 1)
	atomic.StoreInt64(&cooldownUntil, time.Now().Unix()+durSeconds)
	// recreate gate so workers block until it's closed by watcher
	cooldownGate = make(chan struct{})
}

// returns true if cooldown is active
func isCooldownActive() bool {
	return atomic.LoadInt32(&cooldownActive) == 1
}

// returns remaining seconds (>=0)
func cooldownRemaining() int64 {
	if !isCooldownActive() {
		return 0
	}
	rem := atomic.LoadInt64(&cooldownUntil) - time.Now().Unix()
	if rem < 0 {
		return 0
	}
	return rem
}

func main() {
	// ----------------------------
	// Start-up animation
	// ----------------------------
	animStop := make(chan struct{})
	go startAnimation(animStop)

	// ----------------------------
	// Load config
	// ----------------------------
	cfg, err := config.LoadOrCreateConfig("Setting/setting.conf")
	if err != nil {
		close(animStop)
		fmt.Println("\nError loading config:", err)
		os.Exit(1)
	}

	// ----------------------------
	// Load keywords
	// ----------------------------
	keywords, err := domain_generator.LoadKeywordsCSV("Input/Keywords.csv")
	if err != nil {
		close(animStop)
		fmt.Fprintf(os.Stderr, "\nError loading Keywords.csv: %v\n", err)
		os.Exit(1)
	}

	// ----------------------------
	// Generate domains
	// ----------------------------
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

	// ----------------------------
	// Open output CSV
	// ----------------------------
	f, err := os.OpenFile("Input/Malicious_Domains.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		close(animStop)
		fmt.Fprintf(os.Stderr, "Error opening output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	var writerMu sync.Mutex

	// ----------------------------
	// Worker pool
	// ----------------------------
	opts := &wpkg.RunOptions{
		Timeout:         time.Duration(cfg.TimeoutSeconds) * time.Second,
		MaxRetries:      cfg.MaxRetries,
		AutoScale:       cfg.AutoScale,
		MinWorkers:      1,
		NonBlockingLogs: true,
	}
	startWorkers := runtime.NumCPU() * 4
	wp := wpkg.NewWorkerPool(opts, startWorkers)

	// stop startup animation (we're ready)
	close(animStop)
	time.Sleep(150 * time.Millisecond)

	// ----------------------------
	// Runtime bookkeeping
	// ----------------------------
	var completed int64 = 0
	start := time.Now()
	done := make(chan struct{})

	// Rate limiter (requests per second)
	rateLimiter := time.Tick(time.Second / time.Duration(cfg.RateLimit))

	// Progress bar (uses upgraded module)
	pb := progressBar.NewProgressBar(int(total), "Resolving domains", "green")
	// Start auto-render using a small callback so main doesn't manage rendering
	pb.StartAutoRender(func() (int64, int64, bool, int64) {
		cur := atomic.LoadInt64(&completed)
		cd := isCooldownActive()
		rem := cooldownRemaining()
		return cur, total, cd, rem
	})

	// ----------------------------
	// Cooldown watcher: closes gate when cooldown expires
	// ----------------------------
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if isCooldownActive() {
				if time.Now().Unix() >= atomic.LoadInt64(&cooldownUntil) {
					// reset cooldown and open gate to let blocked workers proceed
					atomic.StoreInt32(&cooldownActive, 0)
					// safe to close because we recreate gate when setting cooldown
					close(cooldownGate)
				}
			}
		}
	}()

	// ----------------------------
	// Dispatch DNS tasks
	// ----------------------------
	var wg sync.WaitGroup

	for _, domain := range allGenerated {
		d := domain // capture

		taskFunc := func(ctx context.Context) (interface{}, []string, []string, []error) {
			// If global cooldown active, block until watcher closes the gate
			if isCooldownActive() {
				<-cooldownGate
			}

			// rate limit
			<-rateLimiter

			ok := DNS.Resolve(ctx, d)
			if ok {
				return d, nil, nil, nil
			}
			return nil, nil, nil, nil
		}

		_, resCh, err := wp.SubmitTask(taskFunc, wpkg.Medium, 0)
		if err != nil {
			// failed to submit task: count it as completed and continue
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
				writerMu.Lock()
				_ = writer.Write([]string{s})
				// flush occasionally to avoid large in-memory buffers
				newCount := atomic.LoadInt64(&completed)
				if newCount%100 == 0 {
					writer.Flush()
				}
				writerMu.Unlock()
			}

			// increment counters and update pb
			newCount := atomic.AddInt64(&completed, 1)
			pb.Add(1)

			// Trigger cooldown based on config
			if cfg.CooldownAfter > 0 && newCount%int64(cfg.CooldownAfter) == 0 {
				triggerCooldown(int64(cfg.CooldownDuration))
			}

		}(resCh)
	}

	// wait for workers
	wg.Wait()

	// shutdown
	close(done)
	wp.Stop()

	// finalize writer and progress bar
	writer.Flush()
	pb.StopAutoRender()
	pb.Finish()

	elapsed := time.Since(start).Truncate(time.Millisecond)
	fmt.Printf("\n✔ Resolution complete. Time: %s | Total checked: %d\n", elapsed, total)
	fmt.Println("✔ Valid domains appended to Input/Malicious_Domains.csv")
}
