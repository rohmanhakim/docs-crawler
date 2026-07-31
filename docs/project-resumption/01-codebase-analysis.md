# Codebase Analysis

## Structural Design
- **Current State**: The core crawling logic is currently centralized in `internal/scheduler/scheduler.go` within the `ExecuteCrawlingWithState` method. This method contains a massive `for` loop that sequentially executes every stage of the crawl (fetch -> extract -> sanitize -> convert -> resolve assets -> normalize -> write).
- **The Pipeline Refactor**: There is an incomplete attempt to introduce a proper pipeline architecture in `internal/pipeline/`. The `pipeline.go` file contains placeholder code (`MyWorkflow`, `fetchUser`) which is currently breaking the build. The goal of this package is likely to decouple the stages from the scheduler and allow for better concurrency and testability.
- **Interfaces**: The project makes excellent use of interfaces (e.g., `Fetcher`, `Extractor`, `Sanitizer`) which allows for dependency injection and easier testing (as seen in `NewSchedulerWithDeps`).

## Performance Considerations
- **Concurrency**: Currently, the crawler appears to be strictly sequential. The `ExecuteCrawlingWithState` loop processes one URL at a time from start to finish. To achieve reasonable performance, especially for network-bound tasks like fetching, the pipeline needs to support concurrent workers.
- **Rate Limiting**: A `ratelimiter` is integrated, which is crucial for politeness, but its interaction with a concurrent pipeline will need careful design to avoid bottlenecks while strictly adhering to `robots.txt` delays.

## Security Considerations
- **SSRF (Server-Side Request Forgery)**: The crawler fetches arbitrary URLs discovered on pages. It's critical to ensure that the `urlutil.FilterByHost` and scope enforcement strictly prevent the crawler from accessing internal network resources (e.g., `localhost`, `169.254.169.254`, or internal subnets) unless explicitly configured.
- **HTML Sanitization**: The `htmlSanitizer` stage is vital for security. It must robustly strip malicious scripts, iframes, and object tags before the content is converted to Markdown, preventing XSS if the resulting Markdown is rendered in a web context.
- **Path Traversal**: When saving assets and markdown files locally, the `storageSink` must ensure that filenames derived from URLs do not contain path traversal sequences (`../`) that could overwrite files outside the designated output directory.