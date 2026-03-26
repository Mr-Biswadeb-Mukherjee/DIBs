// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"sync"
	"sync/atomic"
	"time"
)

const qpsHistoryTick = time.Second

type qpsHistoryRecord struct {
	TimestampUTC   string  `json:"timestamp_utc"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	CompletedTotal int64   `json:"completed_total"`
	IntelDoneTotal int64   `json:"intel_done_total"`
	ResolveQPS     float64 `json:"resolve_qps"`
	IntelQPS       float64 `json:"intel_qps"`
}

type qpsHistoryWriter struct {
	start   time.Time
	writer  RecordWriter
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	lastAt  time.Time
	lastRes int64
	lastInt int64

	completed *int64
	intelDone *int64
}

func newQPSHistoryWriter(rt *appRuntime, completed, intelDone *int64) (*qpsHistoryWriter, error) {
	if rt == nil || rt.writers == nil || rt.qpsHistory == "" {
		return nil, nil
	}
	writer, err := newNDJSONWriter(rt.qpsHistory, rt.writers, "qps-history-writer", rt.logErr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &qpsHistoryWriter{
		start:     rt.started,
		writer:    writer,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		lastAt:    now,
		lastRes:   loadCounter(completed),
		lastInt:   loadCounter(intelDone),
		completed: completed,
		intelDone: intelDone,
	}, nil
}

func (q *qpsHistoryWriter) Start() {
	if q == nil {
		return
	}
	q.writeSnapshot(time.Now())
	go q.loop()
}

func (q *qpsHistoryWriter) loop() {
	ticker := time.NewTicker(qpsHistoryTick)
	defer ticker.Stop()
	for {
		select {
		case <-q.stop:
			q.writeSnapshot(time.Now())
			close(q.done)
			return
		case now := <-ticker.C:
			q.writeSnapshot(now)
		}
	}
}

func (q *qpsHistoryWriter) Stop() error {
	if q == nil {
		return nil
	}
	q.once.Do(func() { close(q.stop) })
	<-q.done
	if q.writer == nil {
		return nil
	}
	return q.writer.Close()
}

func (q *qpsHistoryWriter) writeSnapshot(now time.Time) {
	if q == nil || q.writer == nil {
		return
	}
	completed := loadCounter(q.completed)
	intelDone := loadCounter(q.intelDone)
	window := now.Sub(q.lastAt).Seconds()
	record := qpsHistoryRecord{
		TimestampUTC:   now.UTC().Format(time.RFC3339),
		ElapsedSeconds: roundTo2(now.Sub(q.start).Seconds()),
		CompletedTotal: completed,
		IntelDoneTotal: intelDone,
		ResolveQPS:     deltaQPS(completed-q.lastRes, window),
		IntelQPS:       deltaQPS(intelDone-q.lastInt, window),
	}
	q.writer.WriteRecord(record)
	q.lastAt = now
	q.lastRes = completed
	q.lastInt = intelDone
}

func loadCounter(counter *int64) int64 {
	if counter == nil {
		return 0
	}
	return atomic.LoadInt64(counter)
}

func deltaQPS(delta int64, seconds float64) float64 {
	if delta <= 0 || seconds <= 0 {
		return 0
	}
	return roundTo2(float64(delta) / seconds)
}
