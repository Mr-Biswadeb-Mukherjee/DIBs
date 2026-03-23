// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"context"
	"time"

	runtime "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/runtime"
)

type workerPoolFactoryAdapter struct {
	inner WorkerPoolFactory
}

func (a workerPoolFactoryAdapter) NewWorkerPool(
	opts *runtime.WorkerPoolOptions,
	conc int,
	cache runtime.CacheStore,
) runtime.WorkerPool {
	appOpts := &WorkerPoolOptions{
		Timeout:         opts.Timeout,
		MaxRetries:      opts.MaxRetries,
		AutoScale:       opts.AutoScale,
		MinWorkers:      opts.MinWorkers,
		NonBlockingLogs: opts.NonBlockingLogs,
		OnTaskStart:     opts.OnTaskStart,
	}
	if opts.OnTaskFinish != nil {
		appOpts.OnTaskFinish = func(taskID int64, res TaskResult) {
			opts.OnTaskFinish(taskID, runtime.TaskResult{
				Result:   res.Result,
				Info:     res.Info,
				Warnings: res.Warnings,
				Errors:   res.Errors,
			})
		}
	}

	pool := a.inner.NewWorkerPool(appOpts, conc, toAppCacheStore(cache))
	return workerPoolAdapter{inner: pool}
}

type workerPoolAdapter struct {
	inner WorkerPool
}

func (a workerPoolAdapter) SubmitTask(
	f runtime.TaskFunc,
	p runtime.TaskPriority,
	weight int,
) (int64, <-chan runtime.TaskResult, error) {
	id, in, err := a.inner.SubmitTask(func(ctx context.Context) (interface{}, []string, []string, []error) {
		return f(ctx)
	}, TaskPriority(p), weight)
	if err != nil {
		return 0, nil, err
	}

	out := make(chan runtime.TaskResult, 1)
	go func() {
		defer close(out)
		for res := range in {
			out <- runtime.TaskResult{
				Result:   res.Result,
				Info:     res.Info,
				Warnings: res.Warnings,
				Errors:   res.Errors,
			}
		}
	}()
	return id, out, nil
}

func (a workerPoolAdapter) Stop() {
	a.inner.Stop()
}

type writerFactoryAdapter struct {
	inner WriterFactory
}

func (a writerFactoryAdapter) NewNDJSONWriter(
	path string,
	opts runtime.WriterOptions,
) (runtime.RecordWriter, error) {
	appOpts := WriterOptions{
		BatchSize:  opts.BatchSize,
		FlushEvery: opts.FlushEvery,
	}
	if opts.LogHooks.OnError != nil {
		appOpts.LogHooks.OnError = func(err error) {
			opts.LogHooks.OnError(err)
		}
	}

	writer, err := a.inner.NewNDJSONWriter(path, appOpts)
	if err != nil {
		return nil, err
	}
	return recordWriterAdapter{inner: writer}, nil
}

type recordWriterAdapter struct {
	inner RecordWriter
}

func (a recordWriterAdapter) WriteRecord(record interface{}) {
	a.inner.WriteRecord(record)
}

func (a recordWriterAdapter) Close() error {
	return a.inner.Close()
}

type appCacheStoreAdapter struct {
	inner runtime.CacheStore
}

func toAppCacheStore(cache runtime.CacheStore) CacheStore {
	if c, ok := cache.(CacheStore); ok {
		return c
	}
	return appCacheStoreAdapter{inner: cache}
}

func (a appCacheStoreAdapter) Eval(
	ctx context.Context,
	script string,
	keys []string,
	args ...interface{},
) (interface{}, error) {
	return a.inner.Eval(ctx, script, keys, args...)
}

func (a appCacheStoreAdapter) GetValue(ctx context.Context, key string) (string, error) {
	return a.inner.GetValue(ctx, key)
}

func (a appCacheStoreAdapter) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	return a.inner.SetValue(ctx, key, v, ttl)
}

func (a appCacheStoreAdapter) Delete(ctx context.Context, key string) error {
	return a.inner.Delete(ctx, key)
}

func (a appCacheStoreAdapter) RPush(ctx context.Context, key string, values ...string) error {
	return a.inner.RPush(ctx, key, values...)
}

func (a appCacheStoreAdapter) BLPop(ctx context.Context, timeout time.Duration, key string) (string, bool, error) {
	return a.inner.BLPop(ctx, timeout, key)
}

func (a appCacheStoreAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.inner.Expire(ctx, key, ttl)
}

func (a appCacheStoreAdapter) Close() error {
	return a.inner.Close()
}
