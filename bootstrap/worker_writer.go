// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"

	engine "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine"
	filewriter "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/filewriter"
	wpkg "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/worker"
)

type workerPoolFactoryAdapter struct{}

type workerPoolAdapter struct {
	pool *wpkg.WorkerPool
}

func (workerPoolFactoryAdapter) NewWorkerPool(
	opts *engine.WorkerPoolOptions,
	conc int,
	cache engine.CacheStore,
) engine.WorkerPool {
	coreOpts := &wpkg.RunOptions{
		Timeout:         opts.Timeout,
		MaxRetries:      opts.MaxRetries,
		AutoScale:       opts.AutoScale,
		MinWorkers:      opts.MinWorkers,
		NonBlockingLogs: opts.NonBlockingLogs,
		OnTaskStart:     opts.OnTaskStart,
		OnTaskFinish:    mapFinishCallback(opts.OnTaskFinish),
	}
	return &workerPoolAdapter{pool: wpkg.NewWorkerPool(coreOpts, conc, cache)}
}

func mapFinishCallback(fn func(taskID int64, res engine.TaskResult)) func(int64, wpkg.WorkerResult) {
	if fn == nil {
		return nil
	}
	return func(taskID int64, res wpkg.WorkerResult) {
		fn(taskID, engine.TaskResult{
			Result:   res.Result,
			Info:     res.Info,
			Warnings: res.Warnings,
			Errors:   res.Errors,
		})
	}
}

func (w *workerPoolAdapter) SubmitTask(
	f engine.TaskFunc,
	p engine.TaskPriority,
	weight int,
) (int64, <-chan engine.TaskResult, error) {
	id, out, err := w.pool.SubmitTask(wpkg.TaskFunc(f), mapPriority(p), weight)
	if err != nil {
		return 0, nil, err
	}
	resCh := make(chan engine.TaskResult, 1)
	go forwardWorkerResult(out, resCh)
	return id, resCh, nil
}

func mapPriority(p engine.TaskPriority) wpkg.TaskPriority {
	switch p {
	case engine.High:
		return wpkg.High
	case engine.Low:
		return wpkg.Low
	default:
		return wpkg.Medium
	}
}

func forwardWorkerResult(in <-chan wpkg.WorkerResult, out chan<- engine.TaskResult) {
	defer close(out)
	res, ok := <-in
	if !ok {
		return
	}
	out <- engine.TaskResult{
		Result:   res.Result,
		Info:     res.Info,
		Warnings: res.Warnings,
		Errors:   res.Errors,
	}
}

func (w *workerPoolAdapter) Stop() {
	w.pool.Stop()
}

type writerFactoryAdapter struct{}

func (writerFactoryAdapter) NewNDJSONWriter(
	path string,
	opts engine.WriterOptions,
) (engine.RecordWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	coreOpts := filewriter.NDJSONOptions{
		BatchSize:  opts.BatchSize,
		FlushEvery: opts.FlushEvery,
		LogHooks: filewriter.LogHooks{
			OnError: opts.LogHooks.OnError,
		},
	}
	return filewriter.NewNDJSONWriter(path, coreOpts)
}
