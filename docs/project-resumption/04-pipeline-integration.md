# Pipeline Integration Plan

## Goal
Refactor the sequential `ExecuteCrawlingWithState` loop in `internal/scheduler/scheduler.go` into a concurrent, stage-based pipeline using the concepts outlined in `internal/pipeline/`.

## Current Architecture
The `Scheduler` currently owns the entire execution loop. It dequeues a URL, fetches, extracts, sanitizes, converts, resolves, normalizes, and writes sequentially.

## Target Architecture
We need a concurrent worker pool model.
1. **Dispatcher**: Reads from the `Frontier` and sends URLs to a worker pool.
2. **Worker Pool**: A set of goroutines that execute the pipeline stages for a given URL.
3. **Pipeline Stages**: The sequence of operations (Fetch -> Extract -> Sanitize -> Convert -> Resolve -> Normalize -> Write).

## Implementation Steps

1. **Define the Pipeline Interface**:
   Refine `internal/pipeline/pipeline.go` to define a clear interface for a crawler pipeline. It should take a `frontier.CrawlToken` and execute the stages.

2. **Implement the Default Pipeline**:
   Create a `DefaultPipeline` struct that holds references to the `Fetcher`, `Extractor`, `Sanitizer`, etc. Move the logic from the `for` loop in `scheduler.ExecuteCrawlingWithState` into a `Process(ctx context.Context, token frontier.CrawlToken)` method on this pipeline.

3. **Update the Scheduler**:
   Modify `Scheduler.ExecuteCrawlingWithState` to:
   - Initialize a worker pool (e.g., using a channel of `CrawlToken`s).
   - Start `N` worker goroutines (where `N` is configurable, e.g., `cfg.Concurrency()`).
   - Have the main goroutine dequeue from the `Frontier` and send to the worker channel.
   - Handle graceful shutdown and context cancellation.

4. **Concurrency Safety**:
   - Ensure the `Frontier` is safe for concurrent reads (it likely is, but needs verification).
   - Ensure the `RateLimiter` handles concurrent requests correctly (it is named `ConcurrentRateLimiter`, so it should be).
   - Ensure the `FailureJournal` and `StorageSink` are safe for concurrent writes.

5. **Error Handling**:
   The pipeline must return errors back to the scheduler or handle them internally (e.g., logging and continuing, or aborting the worker). The current logic of counting `totalErrors` needs to be thread-safe (e.g., using `atomic.AddInt32`).