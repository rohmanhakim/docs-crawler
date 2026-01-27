# Local Documentation Crawler – Design Document

## 1. Purpose and Scope

This document defines the architecture and design of a **local-only documentation crawler** that converts static documentation websites into **clean, semantically faithful Markdown**, optimized for **LLM Retrieval-Augmented Generation (RAG)** workflows.

The crawler is designed for:
- Local execution on a developer machine
- Polite, low-impact crawling of static documentation sites
- High-quality Markdown output suitable for embedding and retrieval
- Deterministic, repeatable runs

This document is intended as **implementation guidance**, not marketing or end-user documentation.

---

## 2. Non-Goals

The system explicitly does **not** aim to:
- Crawl dynamic, authenticated, or user-specific content
- Execute JavaScript-heavy SPAs (unless explicitly extended)
- Perform semantic rewriting or summarization
- Act as a general-purpose web scraper

---

## 3. High-Level Architecture

```
Static Documentation Site
        │
        ▼
[Crawler + Scheduler]
        │
        ▼
[HTML Fetcher]
        │
        ▼
[DOM Extraction & Cleaning]
        │
        ▼
[HTML → Markdown Converter]
        │
        ▼
[Asset Manager]
        │
        ▼
[Markdown Normalizer]
        │
        ▼
RAG-Ready Markdown Corpus
```

Each stage is isolated and testable. No stage performs multiple responsibilities.

---

## 4. Crawl Strategy

### 4.1 Crawl Scope

- Same-origin URLs only
- Restricted to configurable path prefixes (e.g. `/docs`, `/guide`)
- HTML documents only
- Query parameters stripped unless explicitly allowed
- Fragment identifiers (`#...`) removed

### 4.2 Discovery Method

- Breadth-first traversal
- Optional bootstrap from `sitemap.xml`
- Maximum depth and page count limits enforced

### 4.3 Politeness and Rate Limiting

To avoid IP bans and DDoS detection, the crawler must behave conservatively:

- Low concurrency (default: 1–3 concurrent requests)
- Fixed base delay between requests
- Random jitter applied to delay
- Exponential backoff on HTTP 429 and 5xx responses
- Honor `Retry-After` headers when present

### 4.4 Robots and Safety

- Respect `robots.txt`
- Abort crawl on repeated 403 responses
- Circuit breaker if failure rate exceeds threshold

---

## 5. HTML Fetching Layer

### 5.1 Request Configuration

- Browser-like headers (User-Agent, Accept, Language)
- Automatic redirect handling with hop limits
- Timeouts and retry caps

### 5.2 Error Handling

| Status Code | Action |
|------------|--------|
| 200        | Process page |
| 3xx        | Follow redirect (bounded) |
| 403        | Stop crawling domain |
| 429        | Backoff exponentially |
| 5xx        | Retry with cap |

All fetch results are logged with URL, status, and timing metadata.

---

## 6. DOM Extraction and Cleaning

This stage has the highest impact on downstream RAG quality.

### 6.1 Content Isolation Strategy

Extraction priority order:
1. `<main>`
2. `<article>`
3. Known documentation containers (configurable selectors)
4. Heuristic fallback: largest coherent text block

### 6.2 Content Removal Rules

Remove the following elements:
- Global navigation bars
- Sidebars and menus
- Headers and footers
- Cookie banners
- "Edit on GitHub" links
- Version switchers
- Duplicate tables of contents

### 6.3 Content Preservation Rules

Preserve:
- Heading hierarchy (`h1`–`h6`)
- Code blocks and inline code
- Tables
- Lists
- Admonitions and callouts
- Images and captions

The output DOM should contain **only the document content**, with no site chrome.

---

## 7. HTML to Markdown Conversion

### 7.1 Conversion Principles

- Semantic fidelity over visual fidelity
- No hallucinated structure
- No reformatting of code blocks
- GitHub-Flavored Markdown compatibility

### 7.2 Element Mapping

| HTML Element | Markdown Output |
|-------------|-----------------|
| `h1–h6`     | `#`–`######` |
| `pre > code`| Fenced code blocks with language |
| `table`     | GFM tables |
| `img`       | Relative image references |
| `a`         | Relative links rewritten |
| `blockquote`| `>` |

Inline styles and raw HTML passthrough are avoided.

---

## 8. Asset Management

### 8.1 Image Handling

- Resolve relative image URLs to absolute
- Download images locally
- Preserve original formats (PNG, SVG, GIF, JPG)
- Deduplicate via content hash
- Rewrite Markdown references to local paths

### 8.2 Directory Layout

```
output/
  docs/
    getting-started.md
    api-reference.md
  assets/
    images/
      diagram.png
      flow.svg
```

Broken or missing assets are reported post-run.

---

## 9. Markdown Normalization

### 9.1 Frontmatter

Each Markdown file includes machine-readable metadata:

```yaml
---
title: <Document Title>
source_url: <Canonical URL>
section: <Logical Section>
depth: <Crawl Depth>
---
```

### 9.2 Structural Rules

- Exactly one H1 per file
- No skipped heading levels
- Stable heading anchors (optional)
- No empty sections

### 9.3 RAG-Oriented Constraints

- Sections are chunkable at 500–1,000 tokens
- Code blocks and tables are never split
- Logical topic boundaries preserved

---

## 10. Output and Storage

### 10.1 Primary Output

- Flat Markdown files
- Deterministic filenames
- Stable ordering

### 10.2 Optional Sidecar Metadata

- JSON metadata per document (hashes, crawl time, HTTP headers)
- Useful for change detection and incremental recrawls

---

## 11. Failure Modes and Safeguards

| Risk | Mitigation |
|-----|-----------|
| IP ban | Low concurrency + jitter |
| Infinite crawl | Depth and page caps |
| Duplicate content | Canonical URL hashing |
| Dirty Markdown | DOM-first extraction |
| Missing assets | Verification pass |

---

## 12. Extensibility Considerations

Future extensions may include:
- Versioned documentation awareness
- Incremental recrawling via ETag / Last-Modified
- Sitemap-based prioritization
- LLM-assisted post-processing (optional, offline)

---

## 13. Design Principles Summary

- Deterministic over clever
- Polite over fast
- Semantic accuracy over aesthetics
- RAG quality as the primary success metric

This crawler should be treated as a **content ingestion pipeline**, not a web scraper.
