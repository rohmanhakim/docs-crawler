# URL extraction

**No. Relative URLs must *not* be resolved to absolute URLs in the sanitizer.**

This is a **hard design boundary**, not a preference.

---

## Why the sanitizer must not resolve URLs

Under **Task 3.2**, the sanitizer is a *structural gatekeeper*, not a context-aware rewriter.

From the sanitizer invariants:

> The sanitizer
>
> * MAY repair structure
> * MUST NOT infer meaning
> * MUST NOT make policy decisions
> * MUST NOT invent documents 

Resolving relative → absolute URLs **requires external context**:

* the document’s base URL
* `<base>` tag handling
* canonicalization rules
* crawl scope assumptions

That immediately violates **Invariant S2 — No Semantic Inference** and **Invariant S4 — Repair Is Structural Only**.

---

## What the sanitizer *is* allowed to do with URLs

The sanitizer may:

* **Extract URLs as raw references** (relative or absolute)
* **Preserve them exactly as authored**
* **Deduplicate identical references**
* **Drop malformed URLs** *only if they are structurally invalid* (e.g., not parseable at all)

That’s it.

Think of sanitizer URL handling as:

> *“Report what the DOM says, without interpretation.”*

---

## Where relative → absolute resolution belongs (and why)

### ✅ Scheduler (admission phase)

* Scheduler knows:

  * the fetch URL
  * crawl scope (`allowedHosts`, prefixes)
  * robots context
* Admission decisions *require* absolute URLs
* This is where normalization is **necessary and justified**

### ✅ Asset resolver / Markdown conversion (different purpose)

* These stages rewrite URLs **for output correctness**, not crawl policy
* They already operate with full context
* This is *representation*, not structure validation

---

## Why doing it in sanitizer would be actively harmful

If sanitizer resolves URLs:

* It becomes **context-sensitive**
* It couples Task 3.2 to fetcher/scheduler concerns
* It makes sanitizer outputs **non-deterministic** unless base URL is injected
* It violates the invariant that sanitizer output is provable *from the DOM alone*

Worse: it would make sanitizer failures ambiguous
(“Was this a structural failure or a resolution failure?”)

---

## Correct mental model

| Stage             | URL responsibility               |
| ----------------- | -------------------------------- |
| Extractor         | Sees anchors                     |
| **Sanitizer**     | **Extracts raw references only** |
| Scheduler         | Resolves + admits URLs           |
| Frontier          | Orders URLs                      |
| Assets / Markdown | Rewrites URLs for output         |

---

## Final rule (concise)

> **Sanitizer must treat URLs as opaque strings extracted from DOM.
> No resolution, no rewriting, no canonicalization.**

Anything else is a **Task 3.2 design violation**.
