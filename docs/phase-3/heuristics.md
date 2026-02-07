# Layered Heuristic for Content Extraction

Industry practice converges on a **layered, fallback-based heuristic pipeline** rather than a single “magic rule”. No crawler relies on one technique alone, because documentation sites vary wildly.

Below is the **industry-standard heuristic stack**, in the order it is typically applied.

---

## 1. Semantic container first (highest precision)

**Rule**
If a semantic container exists, treat it as authoritative.

**Selectors (in priority order):**

* `<main>`
* `<article>`
* `[role="main"]`

**Rationale**

* Modern doc frameworks (Docusaurus, MkDocs, GitBook, ReadTheDocs, Sphinx) deliberately wrap primary content in these elements.
* Precision is extremely high; false positives are rare.

**Industry status**
✅ De facto standard
Used by: Mozilla Readability, Diffbot, Goose, newspaper3k

---

## 2. Known documentation container selectors (framework-aware)

**Rule**
If semantic tags are absent or too broad, try **framework-specific containers**.

**Examples**

* `.content`, `.doc-content`, `.markdown-body`
* `#docs-content`, `.rst-content`
* `.theme-doc-markdown`, `.md-content`

**Rationale**

* Documentation generators are consistent *within themselves*
* Configurable allowlist avoids brittle hardcoding

**Industry status**
✅ Common in production crawlers
❗ Must be configurable, not hardcoded

---

## 3. Explicit chrome removal (negative selection)

Instead of trying to *find content*, aggressively remove what is **known not to be content**.

**Elements to strip**

* `nav`, `header`, `footer`, `aside`
* Elements with classes/ids containing:

  * `nav`, `sidebar`, `menu`
  * `breadcrumb`
  * `toc` (except inline TOCs)
  * `search`
  * `footer`, `header`
  * `cookie`, `consent`
  * `version`, `language`, `theme`
  * `edit`, `github`

**Rationale**

* Chrome is repetitive, shallow-text, and structurally obvious
* Removing it simplifies downstream heuristics

**Industry status**
✅ Universal
Used everywhere from crawlers to browser reader modes

---

## 4. Text-density heuristic (fallback, content-agnostic)

If no reliable container exists:

**Compute for each candidate node**

* Total text length
* Text-to-markup ratio
* Paragraph count
* Heading presence (`h1–h3`)
* Code block count

**Select**

> The **largest coherent block of meaningful text**

**Common scoring signals**

* High word count
* Multiple paragraphs
* Presence of headings
* Presence of code blocks (very strong signal for docs)
* Low link density

**Rationale**

* Documentation pages are text-heavy
* Navigation is link-heavy, text-light

**Industry status**
✅ Proven, battle-tested
This is the core of Readability-style algorithms

---

## 5. Structural sanity checks (post-selection validation)

After selecting “main content”, validate it:

**Reject or retry if**

* No headings at all
* Only lists of links
* Only navigation text
* Text length below threshold
* Duplicate content seen elsewhere

**Rationale**

* Prevents false positives
* Protects Markdown normalization invariants

**Industry status**
✅ Standard defensive practice

---

## 6. Deterministic cleanup pass (important for this project)

Once content is isolated:

* Remove empty nodes
* Deduplicate headings
* Normalize heading hierarchy
* Remove repeated inline nav (e.g., page-local breadcrumbs)

This ensures **stable Markdown output**, which is critical for RAG.

---

## Why this layered approach is the standard

| Problem                  | Single heuristic fails | Layered approach succeeds |
| ------------------------ | ---------------------- | ------------------------- |
| Different doc frameworks | ❌                      | ✅                         |
| Missing semantic tags    | ❌                      | ✅                         |
| Weird layouts            | ❌                      | ✅                         |
| Deterministic output     | ❌                      | ✅                         |
| RAG-friendly structure   | ❌                      | ✅                         |

---

## How this maps cleanly to *this* architecture

The current design already matches industry best practice:

| Stage       | Heuristic responsibility                         |
| ----------- | ------------------------------------------------ |
| `extractor` | **Find candidate content container** (Steps 1–4) |
| `sanitizer` | **Strip chrome + normalize DOM** (Steps 3, 6)    |
| `normalize` | **Enforce invariants** (Step 5)                  |

This separation is **correct and intentional**.

---

## One critical industry rule (do not violate)

> **Never rely on CSS class names alone.**
> They are hints, not truth.

Semantic tags + density always win.

---

## TL;DR (industry-standard answer)

> The industry-standard approach is a **layered heuristic pipeline**:
> semantic containers → known content selectors → chrome removal → text-density fallback → structural validation → deterministic cleanup.