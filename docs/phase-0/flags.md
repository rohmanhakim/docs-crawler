Based on **Task 0.1 (configuration & CLI surface)** and the two design documents, the application’s flags naturally fall into **clear functional groups**. Below is a consolidated, implementation-oriented list of **plausible CLI flags**, derived directly from the architecture and configuration responsibilities described in the documents .

---

## 1. Seed & Scope Control Flags

These define **what** the crawler is allowed to visit.

* `--seed-url`
  One or more starting URLs (repeatable or comma-separated)

* `--allowed-host`
  Explicit hostname allowlist (defaults to seed host)

* `--allowed-path-prefix`
  Restrict crawl to paths like `/docs`, `/guide`

* `--disallow-query-params`
  Strip query strings from URLs (default: true)

* `--allow-query-params`
  Explicit allowlist of query parameters

* `--respect-robots`
  Enable robots.txt enforcement (default: true)

---

## 2. Crawl Limits & Safety Flags

These prevent runaway or hostile crawls.

* `--max-depth`
  Maximum link depth from seed URL

* `--max-pages`
  Hard cap on number of pages crawled

* `--max-redirects`
  Redirect hop limit per request

* `--abort-on-403`
  Stop crawl after repeated 403 responses

* `--failure-rate-threshold`
  Circuit breaker threshold (e.g. 30%)

---

## 3. Concurrency, Rate Limiting & Politeness Flags

These are critical to **anti-ban behavior**.

* `--concurrency`
  Number of concurrent fetch workers (default: 1–3)

* `--base-delay-ms`
  Fixed delay between requests (per host)

* `--jitter-ms`
  Random jitter added to base delay

* `--retry-max`
  Maximum retries for recoverable failures

* `--backoff-initial-ms`
  Initial exponential backoff delay

* `--backoff-max-ms`
  Upper bound for backoff delay

* `--honor-retry-after`
  Respect `Retry-After` headers (default: true)

---

## 4. Fetching & HTTP Behavior Flags

Control how requests are made.

* `--user-agent`
  Custom User-Agent string

* `--timeout-ms`
  Per-request timeout

* `--accept-language`
  Value for `Accept-Language` header

* `--max-response-bytes`
  Prevent oversized downloads

---

## 5. DOM Extraction & Cleaning Flags

These tune **content quality**, not crawl behavior.

* `--content-selector`
  CSS selector(s) for main content container

* `--fallback-largest-block`
  Enable heuristic fallback extraction

* `--strip-navigation`
  Remove nav bars and menus

* `--strip-footer`
  Remove footers and legal text

* `--strip-edit-links`
  Remove “Edit on GitHub” links

* `--strip-duplicate-toc`
  Remove redundant tables of contents

---

## 6. Markdown Conversion Flags

Control fidelity and output structure.

* `--markdown-flavor`
  e.g. `gfm` (GitHub-Flavored Markdown)

* `--preserve-html`
  Allow raw HTML passthrough (default: false)

* `--one-h1-per-file`
  Enforce single H1 rule

* `--no-heading-skip`
  Normalize skipped heading levels

---

## 7. Asset Management Flags

For images and other static assets.

* `--download-assets`
  Enable local asset downloading

* `--assets-dir`
  Directory for downloaded assets

* `--dedupe-assets`
  Enable hash-based deduplication

* `--asset-timeout-ms`
  Timeout for asset downloads

* `--fail-on-missing-assets`
  Treat missing assets as fatal

---

## 8. Output & Storage Flags

Define how results are written.

* `--output-dir`
  Root output directory

* `--overwrite`
  Allow overwriting existing output

* `--deterministic-filenames`
  Stable filenames based on URL hashing

* `--write-sidecar-metadata`
  Emit JSON metadata files

---

## 9. RAG-Oriented Normalization Flags

These exist purely for **downstream LLM usage**.

* `--inject-frontmatter`
  Enable YAML frontmatter

* `--frontmatter-fields`
  Select which metadata fields to include

* `--chunk-size-tokens`
  Target chunk size for RAG

* `--no-split-code-blocks`
  Enforce atomic code blocks

* `--no-split-tables`
  Enforce atomic tables

---

## 10. Determinism, Debugging & Observability Flags

Important for development and repeatability.

* `--random-seed`
  Seed for jitter randomness

* `--log-level`
  `debug | info | warn | error`

* `--log-format`
  `text | json`

* `--dry-run`
  Crawl without writing output

* `--dump-dom`
  Write cleaned DOM for debugging

---

## 11. Config File Integration

Typically paired with flags:

* `--config-file`
  Path to YAML/TOML/JSON config file

Flags should **override config file values**, as implied by the configuration layer design .

---

### Summary

For Task **0.1**, the key takeaway is that flags are not arbitrary—they map **directly to subsystem boundaries**:

* Scope & safety
* Politeness & backoff
* Content quality
* Deterministic, RAG-ready output

If you want, next we can:

* Reduce this into a **minimal v1 flag set**, or
* Design a **config schema** that cleanly mirrors these flags.
