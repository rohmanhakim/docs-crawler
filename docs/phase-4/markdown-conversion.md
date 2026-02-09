# Markdown Conversion

**implementing a high-quality HTML → Markdown converter from scratch is overwhelming for a single developer**, *and* **you should still use an existing library**.
The key is **how** you use it, given your architecture and invariants.

Below is the precise recommendation for **Task 4.1**, grounded in your design docs and current codebase.

---

## Why “from scratch” is a bad idea (for this project)

HTML→Markdown conversion looks simple, but in practice it explodes in scope:

* Correct handling of:

  * nested lists
  * tables (GFM alignment rules)
  * code blocks with language hints
  * inline vs block semantics
  * edge cases like `<pre><code>`, `<kbd>`, `<details>`, `<summary>`
* Deterministic output (critical for your RAG goal)
* Avoiding **semantic inference** (which your sanitizer explicitly forbids)

Re-implementing all of this would:

* Take **weeks**, not days
* Duplicate years of bug fixes already solved in mature libraries
* Still be *less correct* than existing solutions

This is **not** where your project’s value lies.

---

## The correct architectural stance (important)

From your design documents, especially §7 and §11:

* **Markdown conversion is a mechanical transformation**
* **All semantics must already be resolved upstream** (extractor + sanitizer)
* The converter must **not “fix” structure**

That means:

> Your HTML→Markdown layer should be a **dumb, deterministic renderer**, not a document interpreter.

This is exactly the kind of problem existing libraries are good at.

---

## Recommended approach (strongly advised)

### 1. Use an existing HTML→Markdown library

### 2. Wrap it with **strict pre- and post-conditions**

Concretely:

* **Sanitizer (Task 3.2)**
  Guarantees:

  * single document
  * stable DOM
  * no ambiguity
    If this fails → hard stop.

* **mdconvert (Task 4.1)**
  Responsibilities:

  * map tags to Markdown
  * preserve order
  * avoid inference
    **No validation here**.

* **normalize (Task 4.2)**
  Enforces:

  * exactly one H1
  * heading constraints
  * frontmatter injection

This aligns perfectly with your current package boundaries.

---

## Libraries that actually fit your constraints (Go)

### ✅ **GitHub / [Johannes Kaufmann’s html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)**

```
go get -u github.com/JohannesKaufmann/html-to-markdown/v2
```

You can:

* Override handlers for `h1–h6`, `pre`, `code`, `table`
* Completely forbid raw HTML passthrough
* Control link and image rewriting

---

## What NOT to do (very important)

❌ Do **not**:

* Infer headings from `<strong>` or CSS
  (that violates Sanitizer Invariant S2) 
* Merge or split sections during conversion
* “Fix” multiple H1s here (that’s normalize’s job)
* Fall back to best-effort Markdown if structure is unclear

If conversion cannot proceed cleanly:

* That’s a **sanitizer bug**, not a markdown problem

---

## How this maps to your current code

Your `mdconvert.Rule.Convert()` is already shaped correctly:

```go
type Rule struct {
	cfg              config.Config
	sanitizedHTMLDoc sanitizer.SanitizedHTMLDoc
}
```

What you should do:

* Feed **only** `SanitizedHTMLDoc` into the library
* Disable:

  * heading inference
  * whitespace “beautification”
* Treat the library as a **pure function**

If the output violates invariants later → `normalize` fails (as designed).

---

## Bottom line (clear recommendation)

* ✅ **Yes, use an existing library**
* ❌ **No, do not implement from scratch**
* ✅ Wrap the library inside your `mdconvert` package
* ✅ Enforce correctness *before* and *after*, not during

This keeps Task 4.1:

* tractable for a solo developer
* aligned with your sanitizer invariants
* faithful to the “deterministic ingestion pipeline” philosophy stated in your design docs