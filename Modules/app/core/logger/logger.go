package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultLogDir       = "Logs"
	defaultChannelSize  = 2048
	defaultBatchSize    = 128
	defaultFlushEvery   = time.Second
	defaultBufferSizeKB = 64 * 1024
)

// Logger writes logs asynchronously so workers stay non-blocking.
type Logger struct {
	filePath string
	entries  chan string

	closeOnce sync.Once
	wg        sync.WaitGroup

	dropped int64

	errMu    sync.Mutex
	closeErr error
}

// NewLogger creates per-module log file under project-root Logs folder.
// Example: NewLogger("dns") -> Logs/dns_2026-03-20_01-10-00.log
func NewLogger(module string) *Logger {
	filePath := buildLogPath(module)
	l := &Logger{
		filePath: filePath,
		entries:  make(chan string, defaultChannelSize),
	}
	l.start()
	return l
}

// Info logs info-level messages.
func (l *Logger) Info(format string, v ...interface{}) {
	l.writeLog("INFO", format, v...)
}

// Warning logs warning-level messages.
func (l *Logger) Warning(format string, v ...interface{}) {
	l.writeLog("WARNING", format, v...)
}

// Alert logs alert-level messages.
func (l *Logger) Alert(format string, v ...interface{}) {
	l.writeLog("ALERT", format, v...)
}

// Close flushes pending logs and closes internal workers.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}

	l.closeOnce.Do(func() {
		close(l.entries)
		l.wg.Wait()
	})

	l.errMu.Lock()
	defer l.errMu.Unlock()
	return l.closeErr
}

// GetFilePath returns the path of the log file.
func (l *Logger) GetFilePath() string {
	if l == nil {
		return ""
	}
	return l.filePath
}

func (l *Logger) writeLog(level, format string, v ...interface{}) {
	if l == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, msg)

	select {
	case l.entries <- logLine:
	default:
		atomic.AddInt64(&l.dropped, 1)
	}
}

func (l *Logger) start() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		l.runWriter()
	}()
}

func (l *Logger) runWriter() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("logger panic recovered path=%s panic=%v\n", l.filePath, r)
		}
	}()

	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("logger open failed path=%s err=%v\n", l.filePath, err)
		l.drainEntries()
		return
	}
	defer file.Close()

	buffer := bufio.NewWriterSize(file, defaultBufferSizeKB)
	ticker := time.NewTicker(defaultFlushEvery)
	defer ticker.Stop()

	batch := make([]string, 0, defaultBatchSize)
	for {
		select {
		case line, ok := <-l.entries:
			if !ok {
				l.flushBatch(buffer, &batch)
				return
			}
			batch = append(batch, line)
			if len(batch) >= defaultBatchSize {
				l.flushBatch(buffer, &batch)
			}

		case <-ticker.C:
			l.flushBatch(buffer, &batch)
		}
	}
}

func (l *Logger) flushBatch(buffer *bufio.Writer, batch *[]string) {
	if len(*batch) == 0 {
		l.appendDroppedNotice(batch)
		if len(*batch) == 0 {
			return
		}
	}

	l.appendDroppedNotice(batch)
	for _, line := range *batch {
		if _, err := buffer.WriteString(line); err != nil {
			l.setCloseError(err)
			break
		}
	}

	if err := buffer.Flush(); err != nil {
		l.setCloseError(err)
	}
	*batch = (*batch)[:0]
}

func (l *Logger) appendDroppedNotice(batch *[]string) {
	n := atomic.SwapInt64(&l.dropped, 0)
	if n <= 0 {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] WARNING: dropped %d log messages due backpressure\n", timestamp, n)
	*batch = append(*batch, line)
}

func (l *Logger) drainEntries() {
	for range l.entries {
	}
}

func (l *Logger) setCloseError(err error) {
	if err == nil {
		return
	}
	l.errMu.Lock()
	if l.closeErr == nil {
		l.closeErr = err
	}
	l.errMu.Unlock()
}

func buildLogPath(module string) string {
	logDir := filepath.Join(".", defaultLogDir)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		logDir = "."
	}
	return filepath.Join(logDir, logFileName(module))
}

func logFileName(module string) string {
	name := sanitizeModuleName(module)
	if strings.HasSuffix(strings.ToLower(name), ".log") {
		return name
	}
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return fmt.Sprintf("%s_%s.log", name, timestamp)
}

func sanitizeModuleName(module string) string {
	if strings.TrimSpace(module) == "" {
		return "module"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	name := replacer.Replace(strings.TrimSpace(module))
	name = strings.Trim(name, "._")
	if name == "" {
		return "module"
	}
	return name
}
