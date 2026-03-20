package filewriter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

const ndjsonBufferSize = 64 * 1024

type NDJSONWriter struct {
	filename string
	file     *os.File
	writer   *bufio.Writer

	rows chan []byte
	wg   sync.WaitGroup

	batchSize  int
	flushEvery time.Duration
	log        LogHooks

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewNDJSONWriter(filename string, opts NDJSONOptions) (*NDJSONWriter, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fw := &NDJSONWriter{
		filename:   filename,
		batchSize:  normalizeBatchSize(opts.BatchSize),
		flushEvery: normalizeFlushEvery(opts.FlushEvery),
		log:        opts.LogHooks,
		ctx:        ctx,
		cancel:     cancel,
	}
	fw.rows = make(chan []byte, fw.batchSize*2)

	if err := fw.open(); err != nil {
		cancel()
		return nil, err
	}

	fw.startAsyncFlusher(fw.rows)
	return fw, nil
}

func normalizeBatchSize(size int) int {
	if size <= 0 {
		return 1
	}
	return size
}

func normalizeFlushEvery(flushEvery time.Duration) time.Duration {
	if flushEvery <= 0 {
		return 500 * time.Millisecond
	}
	return flushEvery
}

func (fw *NDJSONWriter) open() error {
	f, err := os.OpenFile(fw.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	w := bufio.NewWriterSize(f, ndjsonBufferSize)
	fw.file = f
	fw.writer = w
	return nil
}

func (fw *NDJSONWriter) startAsyncFlusher(rows <-chan []byte) {
	fw.wg.Add(1)

	go func() {
		defer fw.wg.Done()
		ticker := time.NewTicker(fw.flushEvery)
		defer ticker.Stop()

		buf := make([][]byte, 0, fw.batchSize)
		for {
			select {
			case row, ok := <-rows:
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
				if len(buf) == 0 {
					continue
				}
				fw.flushBatch(buf)
				buf = buf[:0]
			case <-fw.ctx.Done():
				fw.flushBatch(buf)
				return
			}
		}
	}()
}

func (fw *NDJSONWriter) flushBatch(batch [][]byte) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()
	for _, row := range batch {
		if err := fw.writeLine(row); err != nil && fw.log.OnError != nil {
			fw.log.OnError(err)
		}
	}

	if err := fw.writer.Flush(); err != nil && fw.log.OnError != nil {
		fw.log.OnError(err)
	}
	if fw.log.OnBatchFlush != nil {
		fw.log.OnBatchFlush(len(batch), time.Since(start))
	}
}

func (fw *NDJSONWriter) writeLine(row []byte) error {
	if fw.writer == nil {
		return errors.New("ndjson writer is closed")
	}

	if _, err := fw.writer.Write(row); err != nil {
		return err
	}
	if err := fw.writer.WriteByte('\n'); err != nil {
		return err
	}

	return nil
}

func (fw *NDJSONWriter) WriteRecord(record interface{}) {
	row, err := json.Marshal(record)
	if err != nil {
		if fw.log.OnError != nil {
			fw.log.OnError(err)
		}
		return
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.rows == nil {
		return
	}

	select {
	case fw.rows <- row:
	case <-fw.ctx.Done():
	}
}

func (fw *NDJSONWriter) Close() error {
	fw.mu.Lock()
	if fw.rows == nil {
		fw.mu.Unlock()
		return nil
	}
	close(fw.rows)
	fw.rows = nil
	fw.mu.Unlock()

	fw.wg.Wait()
	fw.cancel()

	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.finalizeLocked()
}

func (fw *NDJSONWriter) finalizeLocked() error {
	if fw.writer != nil {
		if err := fw.writer.Flush(); err != nil {
			return err
		}
		fw.writer = nil
	}

	if fw.file != nil {
		err := fw.file.Close()
		fw.file = nil
		return err
	}
	return nil
}
