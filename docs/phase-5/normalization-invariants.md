# Normalization Invariants

Excellent.
We will define this the same way you defined **Sanitizer Invariants** — normative, non-negotiable, and layer-correct.

This spec assumes:

* Sanitizer invariants already hold 
* Markdown conversion preserved structure faithfully 

Normalize must **not re-interpret structure**, only enforce document contract integrity.

---

## Task 5.1 — Markdown Normalize Invariants

This document defines the **complete, normative invariants** for Markdown normalization.

Any implementation that violates these invariants is incorrect, even if output appears usable.

---

## 1. Scope of Normalize

Normalize is a **document contract gate** between:

* Markdown conversion
* Storage + RAG ingestion

It is **not** a repair stage.
It is **not** a semantic processor.
It is **not** a chunker.

Normalize may:

* Inject metadata
* Enforce structural contracts
* Perform deterministic formatting canonicalization

Normalize must not:

* Infer structure
* Reorder content
* Rewrite meaning
* Improve author intent

---

## 2. Core Responsibility

> Normalize must guarantee that the Markdown document satisfies all structural contracts required for deterministic storage and RAG ingestion.

If any contract cannot be proven, normalize MUST fail.

---

## 3. Document-Level Invariants

---

### Invariant N1 — Exactly One Canonical H1

The document MUST contain exactly one top-level `#` heading.

Violation cases:

* Zero H1
* More than one H1
* H1 appearing after lower-level headings

If violated:

* Return `ErrCauseBrokenH1Invariant`
* Map to `metadata.CauseInvariantViolation`

Normalize MUST NOT invent or merge H1s.

---

### Invariant N2 — Title Binding Consistency

Frontmatter `title` MUST equal the canonical H1 text.

Rules:

* Title is derived from H1 verbatim (trimmed)
* No normalization beyond whitespace trimming
* No casing changes
* No punctuation rewriting

If mismatch occurs, normalize MUST fail.

---

### Invariant N3 — No Skipped Heading Levels

Heading levels must increase by at most +1.

Valid:

```
## → ## → ### → ##
```

Invalid:

```
## → ###   (skip level)
```

If skip detected:

* Fail
* Do not repair

Sanitizer may repair structural hierarchy, but normalize must not.

---

### Invariant N4 — No Orphan Content Outside Root Hierarchy

All content must belong to the document rooted at H1.

Invalid cases:

* Paragraph before first H1
* Blocks after final structural boundary
* Unattached Markdown blocks

If detected → fail.

---

### Invariant N5 — No Empty Sections

A heading must not exist without content before the next heading of same or higher level.

Invalid:

```
### Section A
### Section B
```

Valid:

```
### Section A
Content
```

If empty section detected → fail.

Do not silently remove empty headings.

---

### Invariant N6 — Atomic Block Integrity

The following blocks MUST remain intact:

* Fenced code blocks
* Tables
* Blockquotes

Normalize must verify:

* No heading appears inside a fenced block
* No structural splitting occurred

If atomicity cannot be proven → fail.

---

### Invariant N7 — Deterministic Formatting Canonicalization

Allowed canonicalizations:

* Normalize multiple blank lines to one
* Ensure newline before headings
* Ensure single trailing newline at file end
* Normalize `#Title` → `# Title`

These must be:

* Purely mechanical
* Idempotent
* Order-preserving
* Semantics-preserving

If canonicalization requires guessing intent → fail.

---

## 4. Frontmatter Invariants

---

### Invariant F1 — Frontmatter Must Exist

Normalize MUST inject a frontmatter block.

Minimum required fields:

* `title`
* `source_url`
* `depth`
* `section`

Absence of any required field → fail.

---

### Invariant F2 — Frontmatter Must Be Deterministic

Frontmatter values must be derived solely from:

* Scheduler-provided metadata
* Canonical H1
* Crawl context

It must not:

* Include timestamps
* Include random values
* Include non-deterministic ordering

---

### Invariant F3 — Frontmatter Must Precede Document Content

Frontmatter must:

* Be the first block
* Be delimited correctly
* Be separated from content by exactly one blank line

If document already contains frontmatter (should not happen), normalize must fail.

---

## 5. RAG Structural Guarantees

If normalize succeeds, the document guarantees:

* Single canonical title
* Stable section tree
* Deterministic heading hierarchy
* Chunk boundaries align to headings
* No degenerate empty nodes
* No ambiguous top-level structure

Normalize does not:

* Perform chunking
* Insert chunk markers
* Split content

It only guarantees chunkability.

---

## 6. Error Handling Invariants

---

### Invariant E1 — Contract Violations Are Fatal

All normalize errors are:

* Fatal
* Non-retryable
* Mapped to `metadata.CauseInvariantViolation`

Normalize must never silently degrade.

---

### Invariant E2 — No Partial Emission

If normalize fails:

* No Markdown is emitted
* No artifact is written
* Failure is observable

---

## 7. Explicit Non-Responsibilities

Normalize MUST NOT:

* Repair structural ambiguity (sanitizer responsibility)
* Decide crawl policy
* Rewrite assets
* Deduplicate content
* Perform summarization
* Split documents
* Combine documents

---

## 8. Design Principle (Non-Negotiable)

> Normalize enforces document contracts. It does not fix documents.

If a document cannot satisfy the contract without semantic inference, rejection is correct behavior.

---

## 9. Summary

Task 5.1 is correct if and only if:

* Every emitted Markdown file satisfies all N and F invariants
* No semantic mutation occurred
* No structural reordering occurred
* All failures are explicit and classified as invariant violations

Anything else is a design violation.