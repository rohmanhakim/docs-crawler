# Current State Audit

This document maps the currently implemented packages against the original `docs/tasks.md` to pinpoint exactly where development paused.

## Phase 0 – Project Foundation & Invariants
- **Task 0.1 – Configuration Schema Definition**: ✅ Complete. `internal/config` exists and is well-tested.
- **Task 0.2 – Project Structure & Package Boundaries**: ✅ Complete. The project has a clear `internal/` and `pkg/` structure.
- **Task 0.3 – Logging, Error Classification, and Determinism Rules**: ✅ Complete. `pkg/debug`, `pkg/failure`, and `pkg/failurejournal` are implemented.

## Phase 1 – URL Scope, Frontier, and Scheduling
- **Task 1.1 – URL Normalization and Scope Enforcement**: ✅ Complete. `pkg/urlutil` handles canonicalization.
- **Task 1.2 – Crawl Frontier (BFS, Deduplication, Depth Tracking)**: ✅ Complete. `internal/frontier` is implemented and tested.
- **Task 1.3 – Scheduler and Crawl Lifecycle Control**: ⚠️ Partially Complete. `internal/scheduler` exists and has a sequential execution loop, but it lacks concurrent worker management and proper pipeline integration.

## Phase 2 – Politeness, Robots, and Fetching
- **Task 2.1 – Robots.txt Fetching and Enforcement**: ✅ Complete. `internal/robots` is implemented.
- **Task 2.2 – Rate Limiting, Jitter, and Backoff Policy**: ✅ Complete. Integrated via `github.com/rohmanhakim/rate-limiter`.
- **Task 2.3 – HTML Fetcher with Error Classification**: ✅ Complete. `internal/fetcher` is implemented.

## Phase 3 – DOM Extraction and Sanitization
- **Task 3.1 – Main Content Isolation**: ✅ Complete. `internal/extractor` handles DOM extraction.
- **Task 3.2 – DOM Sanitization and Heading Stabilization**: ✅ Complete. `internal/sanitizer` is implemented.

## Phase 4 – Markdown Conversion and Assets
- **Task 4.1 – HTML to Markdown Semantic Conversion**: ✅ Complete. `internal/mdconvert` is implemented.
- **Task 4.2 – Asset Resolution and Downloading**: ✅ Complete. `internal/assets` handles local asset resolution.

## Phase 5 – Markdown Normalization, Storage, and Validation
- **Task 5.1 – Frontmatter Injection and Structural Validation**: ✅ Complete. `internal/normalize` handles constraints and frontmatter.
- **Task 5.2 – Deterministic Storage and Idempotent Writes**: ✅ Complete. `internal/storage` handles writing results.

## Phase 6 – End-to-End Validation
- **Task 6.1 – Full Crawl Dry Run and Audit**: ❌ Incomplete. The CLI (`cmd/crawler`) needs to be finalized and tested end-to-end.

## Summary
The vast majority of the individual components are built and tested. The primary missing piece is the **Pipeline Integration** (wiring these components together concurrently) and the final **CLI/E2E validation**. The immediate blocker is a build error in an incomplete `internal/pipeline` package.