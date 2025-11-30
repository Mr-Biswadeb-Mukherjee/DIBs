// filewriter_utils.go

package filewriter

import (
	"context"
	"encoding/csv"
	"os"
	"sync"
	"time"
)

// -----------------------------
// INTERNAL IMPLEMENTATION LAYER
// -----------------------------

type CSVWriter struct {
	filename string
	mode     Mode
	atomic   bool

	atomicTemp string

	file   *os.File
	writer *csv.Writer

	rows chan []string
	wg   sync.WaitGroup

	batchSize  int
	flushEvery time.Duration

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	pool sync.Pool
	log  LogHooks
}

func NewWriter(filename string, opts CSVOptions) (*CSVWriter, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fw := &CSVWriter{
		filename: filename,
		mode:     opts.Mode,
		atomic:   opts.Atomic,

		batchSize:  opts.BatchSize,
		flushEvery: opts.FlushEvery,
		log:        opts.LogHooks,

		ctx:    ctx,
		cancel: cancel,

		rows: make(chan []string, opts.BatchSize*2),
		pool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 16)
			},
		},
	}

	if opts.Atomic {
		fw.atomicTemp = filename + ".tmp"
		if err := fw.openAtomic(); err != nil {
			return nil, err
		}
	} else {
		if err := fw.openNormal(); err != nil {
			return nil, err
		}
	}

	fw.startAsyncFlusher()
	return fw, nil
}

// ------------------------
// FILE OPENING
// ------------------------

func (fw *CSVWriter) openNormal() error {
	flags := os.O_CREATE | os.O_WRONLY
	if fw.mode == Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(fw.filename, flags, 0o644)
	if err != nil {
		return err
	}

	fw.file = f
	fw.writer = csv.NewWriter(f)
	return nil
}

func (fw *CSVWriter) openAtomic() error {
	f, err := os.OpenFile(fw.atomicTemp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	fw.file = f
	fw.writer = csv.NewWriter(f)
	return nil
}

// ------------------------
// ASYNC BATCH ENGINE
// ------------------------

func (fw *CSVWriter) startAsyncFlusher() {
	fw.wg.Add(1)

	go func() {
		defer fw.wg.Done()
		ticker := time.NewTicker(fw.flushEvery)
		defer ticker.Stop()

		buf := make([][]string, 0, fw.batchSize)

		for {
			select {
			case row, ok := <-fw.rows:
				if !ok {
					fw.flushBatch(buf)
					return
				}
				buf = append(buf, row)
				if len(buf) >= fw.batchSize {
					fw.flushBatch(buf)
					buf = buf[:0]
				}

			case <-ticker.C:
				if len(buf) > 0 {
					fw.flushBatch(buf)
					buf = buf[:0]
				}

			case <-fw.ctx.Done():
				fw.flushBatch(buf)
				return
			}
		}
	}()
}

func (fw *CSVWriter) flushBatch(batch [][]string) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, row := range batch {
		if err := fw.writer.Write(row); err != nil && fw.log.OnError != nil {
			fw.log.OnError(err)
		}

		row = row[:0]
		fw.pool.Put(row)
	}

	fw.writer.Flush()

	if fw.log.OnBatchFlush != nil {
		fw.log.OnBatchFlush(len(batch), time.Since(start))
	}
}

// ------------------------
// PUBLIC WRITE API
// ------------------------

func (fw *CSVWriter) WriteRow(row []string) {
	buf := fw.pool.Get().([]string)
	buf = append(buf[:0], row...)
	fw.rows <- buf
}

// ------------------------
// CLOSING & CLEANUP
// ------------------------

func (fw *CSVWriter) Close() error {
	fw.mu.Lock()

	if fw.rows == nil {
		fw.mu.Unlock()
		return nil
	}

	close(fw.rows)
	fw.rows = nil
	fw.mu.Unlock()

	fw.wg.Wait()

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.writer != nil {
		fw.writer.Flush()
	}

	if fw.file != nil {
		_ = fw.file.Close()
		fw.file = nil
	}

	if fw.atomic {
		if fw.atomicTemp != "" {
			err := os.Rename(fw.atomicTemp, fw.filename)
			fw.atomicTemp = ""
			return err
		}
	}

	return nil
}
