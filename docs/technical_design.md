# Go-Based Local Documentation Crawler – Technical Design Document

## 1. Purpose

This document provides **technical implementation guidance** for building a **local-only documentation crawler in Go**, based on the previously defined architecture. It focuses on **package boundaries, data flow, concurrency design, and Go-specific considerations**, without prescribing exact code.

The goal is to produce **high-fidelity Markdown output** suitable for **LLM Retrieval-Augmented Generation (RAG)** pipelines, while maintaining **determinism, politeness, and reproducibility**.

---

## 2. Design Constraints

The implementation must:

- Be written entirely in Go
- Run locally as a single binary
- Target static documentation sites only
- Avoid JavaScript execution or headless browsers
- Be deterministic and repeatable
- Respect robots.txt and crawl politeness
- Produce clean, semantically accurate Markdown

---

## 3. High-Level Component Model (Go Packages)

Each major stage is implemented as an isolated Go package with a single responsibility.

```
cmd/
  crawler/

internal/
  config/
  scheduler/
  frontier/
  fetcher/
  robots/
  extractor/
  sanitizer/
  mdconvert/
  assets/
  normalize/
  storage/
  metadata/

pkg/
  urlutil/
  hashutil/
  retry/
  limiter/
```

No package is allowed to depend on a downstream stage.

---

## 4. Configuration Layer

### Responsibilities

- Parse user-provided configuration (file + flags)
- Define crawl scope and limits
- Control rate limiting and backoff behavior

### Key Configuration Domains

- Seed URLs
- Allowed hostnames and path prefixes
- Maximum crawl depth and page count
- Concurrency limits
- Base delay and jitter range
- Retry and backoff parameters
- Output directory layout

Configuration is immutable after startup and passed downward explicitly.

---

## 5. Scheduler and Crawl Frontier

### Scheduler Responsibilities

- Coordinate crawl lifecycle
- Enforce global limits (pages, depth)
- Manage graceful shutdown
- Aggregate crawl statistics

### Frontier Responsibilities

- Maintain BFS ordering
- Deduplicate URLs
- Track crawl depth
- Prevent infinite traversal

### Go-Specific Design

- Use channels to represent work queues
- Use explicit worker pools (not unbounded goroutines)
- Enforce ordering determinism by controlled enqueue logic

The scheduler owns the frontier; workers never enqueue directly.

---

## 6. Politeness, Rate Limiting, and Backoff

### Design Goals

- Avoid IP bans
- Mimic human browsing behavior
- Be conservative by default

### Politeness Model

- Per-host request tracking
- Fixed base delay between requests
- Random jitter added to each delay
- Low default concurrency (1–3)

### Backoff Strategy

- Triggered on HTTP 429 and 5xx responses
- Exponential backoff with upper bound
- Honor Retry-After headers
- Circuit breaker on sustained failures

Backoff state is tracked per host, not globally.

---

## 7. Robots.txt Handling

### Responsibilities

- Fetch robots.txt per host
- Cache rules for crawl duration
- Enforce allow/disallow rules before enqueue

Robots checks occur **before** a URL enters the frontier.

---

## 8. HTML Fetching Layer

### Responsibilities

- Perform HTTP requests
- Apply headers and timeouts
- Handle redirects safely
- Classify responses

### Fetch Semantics

- Only successful HTML responses are processed
- Non-HTML content is discarded
- Redirect chains are bounded
- All responses are logged with metadata

The fetcher never parses content; it only returns bytes and metadata.

---

## 9. DOM Extraction Layer

### Responsibilities

- Parse HTML into a DOM tree
- Isolate main documentation content
- Remove site chrome and noise

### Extraction Strategy

Priority order:
1. Semantic containers (main, article)
2. Configured selectors
3. Heuristic fallback (largest coherent text block)

### Removal Rules

Strip:
- Navigation menus
- Headers and footers
- Sidebars
- Cookie banners
- Version selectors
- Edit links

Only content relevant to the document body may pass through.

---

## 10. Content Sanitization

### Responsibilities

- Normalize malformed markup
- Remove empty or duplicate nodes
- Stabilize heading hierarchy

This stage ensures downstream Markdown conversion is deterministic.

---

## 11. HTML to Markdown Conversion

### Design Principles

- Semantic fidelity over visual fidelity
- No inferred structure
- No code reformatting
- GitHub-Flavored Markdown compatibility

### Conversion Rules

- One H1 per document
- Headings map directly
- Code blocks preserved verbatim
- Tables converted structurally
- Links and images rewritten as relative paths

Inline styles and raw HTML are avoided.

---

## 12. Asset Management

### Responsibilities

- Resolve asset URLs
- Download assets locally
- Deduplicate via content hashing
- Rewrite Markdown references

### Asset Policies

- Preserve original formats
- Stable local filenames
- Separate assets directory
- Missing assets reported, not fatal

---

## 13. Markdown Normalization

### Responsibilities

- Inject frontmatter
- Enforce structural rules
- Prepare documents for RAG chunking

### Frontmatter Fields

- Title
- Source URL
- Crawl depth
- Section or category

### RAG-Oriented Constraints

- Logical section boundaries preserved
- Code blocks and tables are atomic
- Chunk sizes predictable

---

## 14. Storage Layer

### Responsibilities

- Persist Markdown files
- Write assets
- Ensure deterministic filenames

### Output Characteristics

- Stable directory layout
- Idempotent writes
- Overwrite-safe reruns

---

## 15. Metadata and Observability

### Metadata Collected

- Fetch timestamps
- HTTP status codes
- Content hashes
- Crawl depth

### Logging Goals

- Debuggable crawl behavior
- Post-run auditability
- Failure diagnostics

Structured logging is preferred.

---

## 16. Error Handling Philosophy

- Fail fast on configuration errors
- Skip and log malformed pages
- Abort crawl on hostile responses
- Never silently drop content

Errors are classified as:
- Fatal
- Recoverable
- Informational

---

## 17. Determinism Guarantees

The system must ensure:

- Stable crawl order
- Stable output filenames
- Stable Markdown formatting
- No time-based nondeterminism beyond logging

Randomness (jitter) must be seedable.

---

## 18. Extensibility

Future extensions may include:

- Incremental recrawling
- Sitemap prioritization
- Version-aware documentation trees
- Offline LLM-assisted post-processing

These must not compromise core determinism.

---

## 19. Summary

This Go-based design treats documentation crawling as a **content ingestion pipeline**, not a scraper. By enforcing strict package boundaries, conservative crawling behavior, and deterministic transformations, the resulting Markdown corpus is well-suited for **high-quality RAG workflows**.

Go’s strengths in concurrency control, static binaries, and predictable execution make it a strong and appropriate choice for this system.
