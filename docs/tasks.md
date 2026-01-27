# Development Phases and Iterations Plan

This document breaks down the implementation of the **Local Documentation Crawler** into sequential development phases and iterations. It is derived directly from the approved *Design Document* and *Technical Design Document* and is intended to guide execution, testing, and review.

The plan emphasizes:
- Deterministic progress
- Testable outcomes per task
- Explicit dependency management
- Incremental validation aligned with RAG quality goals

---

## Phase 0 – Project Foundation & Invariants

### Objective
Establish the non-negotiable foundations: configuration, determinism guarantees, project structure, and observability. No crawling logic is implemented yet.

---

### Task 0.1 – Configuration Schema Definition

**Background**  
The crawler’s behavior must be fully defined at startup and remain immutable throughout execution. Configuration governs crawl scope, politeness, and output layout.

**BDD Scenario**  
- **Given** a user provides a configuration file and CLI flags  
- **When** the application starts  
- **Then** a validated, immutable configuration object is produced or the process exits with a clear error

**Expected Testable Output**  
- Documented configuration fields and defaults  
- Validation rules for invalid or conflicting options  
- Deterministic configuration snapshot logged at startup

**Blocking / Blocked-by**  
- Blocks all subsequent phases

**Expected Duration**  
- 0.5–1 day

---

### Task 0.2 – Project Structure & Package Boundaries

**Background**  
Strict package boundaries are required to prevent responsibility leakage and to preserve pipeline determinism.

**BDD Scenario**  
- **Given** the project is initialized  
- **When** packages are created according to the technical design  
- **Then** no package depends on a downstream stage

**Expected Testable Output**  
- Directory and package layout matching the technical design  
- Dependency graph review showing no illegal imports

**Blocking / Blocked-by**  
- Blocked by: Task 0.1  
- Blocks all crawler logic phases

**Expected Duration**  
- 0.5 day

---

### Task 0.3 – Logging, Error Classification, and Determinism Rules

**Background**  
Observability and determinism rules must be defined before behavior is implemented, not retrofitted.

**BDD Scenario**  
- **Given** any component emits logs or errors  
- **When** the crawler runs twice with the same inputs  
- **Then** behavior and outputs are identical (excluding timestamps)

**Expected Testable Output**  
- Error taxonomy (fatal / recoverable / informational)  
- Structured logging fields defined  
- Documented determinism guarantees

**Blocking / Blocked-by**  
- Blocked by: Task 0.2  
- Blocks Phase 1+

**Expected Duration**  
- 0.5 day

---

## Phase 1 – URL Scope, Frontier, and Scheduling

### Objective
Implement controlled discovery without fetching content yet. This phase proves crawl safety and boundedness.

---

### Task 1.1 – URL Normalization and Scope Enforcement

**Background**  
URL normalization prevents duplicate crawling and infinite traversal due to fragments or query noise.

**BDD Scenario**  
- **Given** multiple URLs differing only by fragments or queries  
- **When** they are processed for enqueue  
- **Then** they normalize to a single canonical URL or are rejected

**Expected Testable Output**  
- Canonical URL rules documented  
- Test cases proving duplicate suppression

**Blocking / Blocked-by**  
- Blocked by: Phase 0

**Expected Duration**  
- 1 day

---

### Task 1.2 – Crawl Frontier (BFS, Deduplication, Depth Tracking)

**Background**  
The frontier guarantees breadth-first traversal and bounded exploration.

**BDD Scenario**  
- **Given** seed URLs with allowed scope  
- **When** URLs are enqueued and dequeued  
- **Then** traversal order is BFS and depth limits are enforced

**Expected Testable Output**  
- Deterministic dequeue order  
- Depth and page-count caps enforced

**Blocking / Blocked-by**  
- Blocked by: Task 1.1  
- Blocks: Scheduler execution

**Expected Duration**  
- 1–1.5 days

---

### Task 1.3 – Scheduler and Crawl Lifecycle Control

**Background**  
The scheduler coordinates workers, enforces limits, and owns crawl termination.

**BDD Scenario**  
- **Given** a finite frontier and configured limits  
- **When** the crawl executes  
- **Then** workers stop deterministically when limits are reached

**Expected Testable Output**  
- Graceful shutdown behavior  
- Accurate crawl statistics

**Blocking / Blocked-by**  
- Blocked by: Task 1.2  
- Blocks Phase 2

**Expected Duration**  
- 1 day

---

## Phase 2 – Politeness, Robots, and Fetching

### Objective
Ensure the crawler behaves safely and conservatively before processing content.

---

### Task 2.1 – Robots.txt Fetching and Enforcement

**Background**  
Robots rules must be enforced before URLs enter the frontier.

**BDD Scenario**  
- **Given** a site disallows certain paths  
- **When** URLs are evaluated for enqueue  
- **Then** disallowed URLs never enter the frontier

**Expected Testable Output**  
- Cached robots rules per host  
- Verified allow/deny decisions

**Blocking / Blocked-by**  
- Blocked by: Phase 1

**Expected Duration**  
- 0.5–1 day

---

### Task 2.2 – Rate Limiting, Jitter, and Backoff Policy

**Background**  
Politeness is critical to avoid IP bans and hostile responses.

**BDD Scenario**  
- **Given** repeated requests to the same host  
- **When** delays and jitter are applied  
- **Then** request timing stays within configured bounds

**Expected Testable Output**  
- Measurable delay enforcement  
- Exponential backoff on 429 / 5xx

**Blocking / Blocked-by**  
- Blocked by: Task 2.1  
- Blocks actual fetching

**Expected Duration**  
- 1 day

---

### Task 2.3 – HTML Fetcher with Error Classification

**Background**  
Fetching must be isolated from parsing and strictly classified.

**BDD Scenario**  
- **Given** various HTTP responses  
- **When** a page is fetched  
- **Then** only valid HTML responses proceed downstream

**Expected Testable Output**  
- Logged fetch metadata  
- Correct retry / abort behavior

**Blocking / Blocked-by**  
- Blocked by: Task 2.2  
- Blocks Phase 3

**Expected Duration**  
- 1 day

---

## Phase 3 – DOM Extraction and Sanitization

### Objective
Produce clean, content-only DOM trees suitable for Markdown conversion.

---

### Task 3.1 – Main Content Isolation

**Background**  
Poor extraction directly degrades RAG quality.

**BDD Scenario**  
- **Given** a documentation HTML page  
- **When** content is extracted  
- **Then** navigation and chrome are removed, and main content remains

**Expected Testable Output**  
- DOM snapshots containing only document content

**Blocking / Blocked-by**  
- Blocked by: Phase 2

**Expected Duration**  
- 1–2 days

---

### Task 3.2 – DOM Sanitization and Heading Stabilization

**Background**  
Malformed or inconsistent markup must be normalized.

**BDD Scenario**  
- **Given** inconsistent heading levels  
- **When** sanitization runs  
- **Then** heading hierarchy is valid and deterministic

**Expected Testable Output**  
- Sanitized DOM with stable structure

**Blocking / Blocked-by**  
- Blocked by: Task 3.1  
- Blocks Phase 4

**Expected Duration**  
- 1 day

---

## Phase 4 – Markdown Conversion and Assets

### Objective
Transform clean DOM into high-fidelity, RAG-ready Markdown.

---

### Task 4.1 – HTML to Markdown Semantic Conversion

**Background**  
Markdown output must preserve semantics without hallucination.

**BDD Scenario**  
- **Given** a sanitized DOM  
- **When** it is converted to Markdown  
- **Then** headings, code, tables, and lists are preserved exactly

**Expected Testable Output**  
- GFM-compatible Markdown files

**Blocking / Blocked-by**  
- Blocked by: Phase 3

**Expected Duration**  
- 1–1.5 days

---

### Task 4.2 – Asset Resolution and Downloading

**Background**  
Images and other assets must be local and deduplicated.

**BDD Scenario**  
- **Given** Markdown referencing images  
- **When** assets are processed  
- **Then** references point to local, hashed files

**Expected Testable Output**  
- Assets directory populated  
- Rewritten Markdown links

**Blocking / Blocked-by**  
- Blocked by: Task 4.1  
- Blocks Phase 5

**Expected Duration**  
- 1 day

---

## Phase 5 – Markdown Normalization, Storage, and Validation

### Objective
Finalize output for RAG consumption and ensure repeatability.

---

### Task 5.1 – Frontmatter Injection and Structural Validation

**Background**  
Metadata is required for traceability and chunking.

**BDD Scenario**  
- **Given** a Markdown document  
- **When** normalization runs  
- **Then** frontmatter and structural rules are enforced

**Expected Testable Output**  
- Markdown with valid frontmatter  
- Exactly one H1 per file

**Blocking / Blocked-by**  
- Blocked by: Phase 4

**Expected Duration**  
- 0.5–1 day

---

### Task 5.2 – Deterministic Storage and Idempotent Writes

**Background**  
Re-running the crawler must not introduce drift.

**BDD Scenario**  
- **Given** two identical crawl runs  
- **When** output is written  
- **Then** files are byte-for-byte identical

**Expected Testable Output**  
- Stable filenames  
- Repeatable output

**Blocking / Blocked-by**  
- Blocked by: Task 5.1

**Expected Duration**  
- 0.5 day

---

## Phase 6 – End-to-End Validation

### Objective
Prove the system meets RAG ingestion quality standards.

---

### Task 6.1 – Full Crawl Dry Run and Audit

**Background**  
An end-to-end crawl validates all assumptions.

**BDD Scenario**  
- **Given** a known static documentation site  
- **When** the crawler runs end-to-end  
- **Then** the output corpus is complete, clean, and deterministic

**Expected Testable Output**  
- Audit checklist passed  
- Crawl report and metrics

**Blocking / Blocked-by**  
- Blocked by: Phase 5

**Expected Duration**  
- 1 day

---

**End of Document**
