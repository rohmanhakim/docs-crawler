# Sanitization

**Heading stabilization is necessary, but it is *not* the only concern of task 3.2**.

Task 3.2 is the **HTML sanitization stage**, and *heading stabilization is only one responsibility inside it*, not the whole task.

---

## What task 3.2 is *actually* responsible for

Per the design and technical docs, task 3.2 corresponds to the **`sanitizer` package** and sits **between extraction and Markdown conversion**. Its role is to make the DOM **safe, minimal, and deterministic** for downstream stages.

From the Technical Design (§10) and the sanitizer module comments, task 3.2 responsibilities are:

### 1. Malformed DOM normalization

Sanitizer is the *last chance* to deal with HTML pathologies such as:

* Unclosed tags
* Invalid nesting
* Browser-recovered DOM quirks
* Broken nodes introduced by aggressive site templates

If this isn’t handled here, `mdconvert` becomes nondeterministic.

---

### 2. Removal of empty and duplicate nodes

Extraction already removes *chrome*, but sanitizer must:

* Drop empty wrappers (`<div></div>`, empty `<section>`)
* Collapse duplicated blocks caused by responsive layouts
* Eliminate repeated headings or paragraphs that survived extraction

This is explicitly called out as sanitizer responsibility, not extractor.

---

### 3. Heading stabilization (partial)

This includes:

* Fixing broken heading hierarchies (e.g. `h1 → h3` skips)
* Normalizing multiple `h1`s into a single document root heading
* Ensuring deterministic ordering and nesting
* Preventing downstream invariant violations (which are enforced later in `normalize`)

This is **structural repair**, not semantic rewriting.

---

### 4. Determinism enforcement

Task 3.2 is where the system ensure:

* Same input HTML → same sanitized DOM
* No order instability due to traversal artifacts
* No dependence on browser heuristics or parser randomness

This is why sanitizer exists as a **separate stage** instead of being merged into extraction or markdown conversion.

---

## What task 3.2 is **not** responsible for

To avoid scope creep:

| Concern                                        | Stage                    |
| ---------------------------------------------- | ------------------------ |
| Chrome removal (nav, footer, sidebar)          | **Extractor (task 3.1)** |
| Content selection / “largest coherent block”   | **Extractor (task 3.1)** |
| Markdown formatting rules                      | **mdconvert**            |
| Enforcing “exactly one H1” as a hard invariant | **normalize**            |
| Chunking for RAG                               | **normalize**            |
| Asset URL rewriting                            | **assets**               |

The sanitizer **prepares**, it does not **decide policy**.

---

## Mental model to use

> **Extractor answers:** *“What is the document?”*
> **Sanitizer answers:** *“Is this document structurally safe and deterministic?”*
> **Normalizer answers:** *“Does this document satisfy output invariants?”*

Heading stabilization lives in the *middle* layer for a reason.

---

## Practical guidance for our implementation

For task 3.2, we should implement **at least**:

* A heading normalization pass
* A DOM cleanup pass (empty / duplicate nodes)
* A malformed-markup tolerance strategy
* Deterministic traversal / rewrite rules

If we *only* implement heading stabilization, we are **under-implementing task 3.2**, even if later stages appear to work.