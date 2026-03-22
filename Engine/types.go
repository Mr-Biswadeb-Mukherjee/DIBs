// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package engine

import app "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app"

type Dependencies = app.Dependencies
type Config = app.Config
type Paths = app.Paths
type Startup = app.Startup
type ModuleLogger = app.ModuleLogger
type LogSet = app.LogSet
type EvalStore = app.EvalStore
type CacheStore = app.CacheStore
type RateLimiter = app.RateLimiter
type CooldownManager = app.CooldownManager
type CooldownFactory = app.CooldownFactory
type TaskFunc = app.TaskFunc
type TaskResult = app.TaskResult
type TaskPriority = app.TaskPriority
type WorkerPoolOptions = app.WorkerPoolOptions
type WorkerPool = app.WorkerPool
type WorkerPoolFactory = app.WorkerPoolFactory
type WriterLogHooks = app.WriterLogHooks
type WriterOptions = app.WriterOptions
type RecordWriter = app.RecordWriter
type WriterFactory = app.WriterFactory
type AdaptiveSnapshot = app.AdaptiveSnapshot
type AdaptiveDecision = app.AdaptiveDecision
type AdaptiveController = app.AdaptiveController
type AdaptiveFactory = app.AdaptiveFactory

const (
	Low    = app.Low
	Medium = app.Medium
	High   = app.High
)
