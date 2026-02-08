# Task 3.2 — Sanitizer Invariants

This document defines the **complete, normative invariants** for **Task 3.2 (HTML Sanitization)**.

These invariants are **design constraints**, not implementation suggestions. Any sanitizer implementation that violates them is **incorrect**, even if downstream stages appear to work.

---

## 1. Scope of the Sanitizer

The sanitizer is a **structural gatekeeper** between:

- **Extractor (task 3.1)** — which isolates *what* content belongs to the document
- **Markdown conversion** — which assumes the structure is already valid and deterministic

The sanitizer:
- MAY repair structure
- MUST NOT infer meaning
- MUST NOT invent documents
- MUST NOT make policy decisions

---

## 2. Core Responsibility

> **The sanitizer must ensure that a single, deterministic document structure exists and can be proven without semantic inference.**

If this cannot be guaranteed, the sanitizer MUST fail with `ErrCauseStructurallyIncorrect`.

---

## 3. Fundamental Invariants

### Invariant S1 — Deterministic Structure

The sanitized DOM MUST represent **exactly one** document whose structure is:

- Linearizable into a single reading order
- Independent of visual layout (CSS, grid, columns)
- Independent of viewport or device assumptions

If more than one plausible structure exists → **fail**.

---

### Invariant S2 — No Semantic Inference

Sanitizer transformations MUST NOT:

- Rename headings
- Invent missing headings
- Merge sections based on meaning
- Reorder content to "improve flow"
- Choose between competing interpretations

If a repair requires guessing author intent → **fail**.

---

### Invariant S3 — Single Document Root

The sanitizer MUST be able to identify a **single document root** such that:

- All headings belong to the same logical hierarchy
- There is at most one plausible top-level root

If multiple competing roots exist → **fail**.

---

### Invariant S4 — Repair Is Structural Only

Allowed repairs include:

- Fixing malformed or unclosed tags
- Normalizing invalid nesting
- Renumbering heading levels **without reordering nodes**
- Removing empty containers
- Removing duplicate nodes with identical structure and content

Repairs MUST preserve original content order.

---

### Invariant S5 — No Document Synthesis

The sanitizer MUST NOT:

- Split a page into multiple documents
- Combine independent sections into a new unit
- Create new provenance boundaries

Each input page produces either:
- **One sanitized document**, or
- **A sanitizer error**

---

## 4. Heading-Specific Invariants

### Invariant H1 — Repairable Heading Hierarchy

The sanitizer MAY repair heading hierarchies if and only if:

- Repair does not require reordering nodes
- Repair does not require inventing hierarchy
- Repair preserves the relative order of content

Example (allowed):
```
h1 → h3 → h2  →  h1 → h2 → h3
```

---

### Invariant H2 — Ambiguous Roots Are Fatal

If the document contains multiple `h1` elements **without a provable primary root**, sanitizer MUST fail.

Examples that MUST fail:
- Product comparison pages
- Hub pages with parallel top-level sections
- Multi-version documents without explicit scoping

---

### Invariant H3 — Headings Must Anchor Structure

If no headings exist and no equivalent structural anchors can be proven, sanitizer MUST fail.

Adding headings is semantic invention and forbidden.

---

## 5. Linearization Invariant

### Invariant L1 — Stable Reading Order

The sanitizer MUST be able to linearize the document such that:

- Order is derivable purely from DOM structure
- No CSS-based assumptions are required
- No layout heuristics are required

If reading order depends on presentation → **fail**.

---

## 6. Error Invariants

### Invariant E1 — Broken DOM

`ErrCauseStructurallyIncorrect` MUST be returned when:

- Structure is ambiguous
- Multiple valid interpretations exist
- Repair would require semantic inference

This error is:
- **Fatal**
- **Non-retryable**
- **Mapped to `metadata.CauseContentInvalid`**

---

### Invariant E2 — Unparseable HTML (Optional)

If no DOM can be constructed at all, sanitizer MAY return `ErrCauseUnparseableHTML`.

This represents unreadable structure, not ambiguity.

---

## 7. Explicit Non-Responsibilities

The sanitizer MUST NOT:

- Enforce "exactly one H1" (normalize does this)
- Decide RAG chunking boundaries
- Rewrite links or assets
- Apply crawl or storage policy
- Emit best-effort approximations

---

## 8. Output Guarantees

If sanitizer succeeds, it guarantees:

- Exactly one deterministic document
- Stable structure across runs
- Safe downstream Markdown conversion
- No invented semantics

If sanitizer fails, it guarantees:

- No partial or corrupt document is emitted
- Failure reason is explicit and observable

---

## 9. Design Principle (Non-Negotiable)

> **It is always better to reject an ambiguous document than to invent a wrong one.**

This invariant supersedes coverage, convenience, and aesthetics.

---

## 10. Summary

Task 3.2 is successful if and only if:

- All accepted documents have provable structure
- All rejected documents are rejected *because structure could not be proven*
- No sanitizer logic depends on meaning, intent, or preference

Any deviation is a design violation.

