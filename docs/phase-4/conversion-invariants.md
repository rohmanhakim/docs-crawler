# Task 4.1 — Markdown Conversion Invariants (`mdconvert`)

## 1. Scope and Role

`mdconvert` is a **pure rendering stage** that transforms a **sanitized, deterministic HTML document** into **GitHub-Flavored Markdown (GFM)**.

It sits **after** the sanitizer and **before** normalization.

> **mdconvert assumes structure is already proven.
> mdconvert must not attempt to prove or repair structure.**

---

## 2. Fundamental Contract

### Invariant M0 — Single-Input Assumption

`mdconvert` MUST operate **only** on a `SanitizedHTMLDoc`.

* It MUST NOT accept raw HTML
* It MUST NOT inspect extractor output directly
* It MUST trust all structural guarantees provided by the sanitizer

If sanitizer guarantees are violated, the behavior of `mdconvert` is undefined and **not its responsibility**.

---

## 3. Non-Inference Rule (Critical)

### Invariant M1 — No Semantic Inference

`mdconvert` MUST NOT:

* Infer headings from `<strong>`, `<b>`, `<span>`, or CSS
* Promote or demote headings based on visual style
* Invent section boundaries
* Merge or split sections
* Guess document intent

**Every Markdown construct must correspond to an explicit HTML node.**

If a Markdown construct cannot be mapped mechanically → it MUST NOT be emitted.

---

## 4. Order and Determinism

### Invariant M2 — Stable Linearization

The Markdown output MUST:

* Preserve the exact DOM traversal order
* Reflect a single, stable reading order
* Be independent of:

  * CSS
  * layout
  * viewport assumptions

No reordering is permitted under any circumstance.

---

### Invariant M3 — Deterministic Output

Given identical `SanitizedHTMLDoc` input:

* Markdown output MUST be byte-for-byte identical across runs
* No time-based, random, or environment-dependent formatting is allowed
* Whitespace normalization MUST be deterministic

This invariant is required for:

* reproducible crawls
* stable diffs
* RAG consistency

---

## 5. Element Mapping Invariants

### Invariant M4 — Explicit Element Mapping Only

Only the following mappings are allowed:

| HTML            | Markdown          |
| --------------- | ----------------- |
| `h1–h6`         | `#–######`        |
| `p`             | paragraph         |
| `pre > code`    | fenced code block |
| `code` (inline) | inline backticks  |
| `ul / ol / li`  | Markdown lists    |
| `table`         | GFM table         |
| `blockquote`    | `>`               |
| `a`             | Markdown link     |
| `img`           | Markdown image    |

If an HTML element does not have a defined mapping:

* It MUST be ignored **or**
* Its text content may be emitted (without structure)

Raw HTML passthrough is **forbidden**.

---

### Invariant M5 — Code Fidelity

For code blocks:

* Content MUST be preserved verbatim
* No re-indentation
* No trimming
* No syntax rewriting
* Language fences may only be added if explicitly present in HTML

Example:

```html
<pre><code class="language-go">...</code></pre>
```

→

````
```go
...
```
````

No guessing allowed.

---

### Invariant M6 — Table Structural Fidelity

Tables MUST be converted structurally:

* Column order preserved
* Cell content preserved
* No column alignment inference
* No column dropping or merging

If a table cannot be represented in GFM **without inference**, it MUST still be emitted mechanically (even if ugly).

---

## 6. Heading-Specific Invariants

### Invariant M7 — No Heading Repair

`mdconvert` MUST NOT:

* Enforce “exactly one H1”
* Repair skipped heading levels
* Resolve multiple H1s
* Decide document roots

All heading validity rules belong to **normalize**, not `mdconvert`.

---

### Invariant M8 — Heading Identity Preservation

Each HTML heading node maps to **exactly one** Markdown heading.

* No deduplication
* No collapsing
* No renaming
* No re-leveling

---

## 7. Links and Images

### Invariant M9 — Reference Preservation

Links and images MUST:

* Preserve original text/alt text
* Preserve relative vs absolute nature
* NOT be resolved, downloaded, or rewritten here

Asset resolution is **strictly downstream**.

---

## 8. Failure Semantics

### Invariant M10 — mdconvert Never Rejects Structure

`mdconvert` MUST NOT fail due to:

* Multiple H1s
* Awkward heading order
* Long documents
* “Ugly” Markdown

If sanitizer succeeded, `mdconvert` MUST emit Markdown.

Structural rejection is **not allowed** at this stage.

---

## 9. Explicit Non-Responsibilities

`mdconvert` MUST NOT:

* Validate document invariants
* Inject frontmatter
* Chunk content
* Decide RAG boundaries
* Rewrite URLs
* Download assets
* Emit metadata
* Emit errors for content correctness

Those belong to:

* sanitizer (structure proof)
* normalize (invariants)
* assets (resolution)
* storage (persistence)

---

## 10. Output Guarantees

If `mdconvert` succeeds, it guarantees:

* Markdown is:

  * deterministic
  * mechanically derived
  * semantically neutral
* All downstream validation can operate **without HTML context**

---

## 11. Design Principle (Authoritative)

> **mdconvert is a renderer, not a document editor.**

If Markdown looks bad but is faithful, `mdconvert` is correct.
If Markdown looks good because structure was inferred, `mdconvert` is wrong.