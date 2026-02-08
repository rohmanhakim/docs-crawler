# Structural Repair

A **hard boundary on what task 3.2 (sanitizer) is allowed to do**.

---

## Structural repair: what it *means*

**Structural repair** fixes the *shape* of the document **without changing what the document is saying**.

Think in terms of **tree invariants**, not meaning.

Examples of **structural repair** (allowed in task 3.2):

* Fixing invalid or skipped heading levels

  ```html
  <h1>Title</h1>
  <h3>Usage</h3>
  ```

  → repaired to:

  ```html
  <h1>Title</h1>
  <h2>Usage</h2>
  ```

* Collapsing duplicate headings caused by layout artifacts

  ```html
  <h2>API</h2>
  <h2>API</h2>
  ```

  → keep one

* Removing empty wrappers

  ```html
  <section><div></div></section>
  ```

  → removed

* Normalizing malformed DOM (unclosed tags, bad nesting)

* Ensuring headings form a valid tree (`h1 → h2 → h3`, no jumps)

These operations **do not invent, remove, or reinterpret content** — they only make the DOM **well-formed and deterministic**.

---

## Semantic rewriting: what it is *not*

**Semantic rewriting** changes *meaning*, *intent*, or *information content*.

Examples of **semantic rewriting** (NOT allowed in task 3.2):

* Renaming headings

  > “Overview” → “Introduction”

* Merging sections because they “look similar”

* Reordering sections for “better flow”

* Inferring missing headings

  > “This paragraph looks like a heading”

* Summarizing or paraphrasing text

* Collapsing content because it seems redundant *semantically*

These require **interpretation** and **judgment** — exactly what the design explicitly avoids.

The crawler is a **content ingestion pipeline**, not an editor.

---

## Why this boundary matters in this project

The architecture deliberately separates responsibilities:

| Stage                     | Allowed to interpret meaning? |
| ------------------------- | ----------------------------- |
| extractor                 | no                            |
| sanitizer (task 3.2)      | no                            |
| mdconvert                 | no                            |
| normalize                 | no                            |
| (future LLM post-process) | maybe                         |

Task 3.2 exists **only** to ensure downstream stages:

* behave deterministically
* don’t crash on malformed input
* don’t violate structural invariants accidentally

If sanitizer starts doing semantic work, we break:

* determinism
* reproducibility
* debuggability
* RAG faithfulness

---

## A concrete rule of thumb

When implementing task 3.2, ask this for every transformation:

> **“Would two humans reasonably disagree about this change?”**

* If **yes** → semantic → not sanitizer
* If **no** → structural → sanitizer

Example:

* “`h3` cannot follow `h1` directly” → no disagreement → structural
* “This section should be merged with the previous one” → subjective → semantic

---

## How this maps to the codebase

In the `sanitizer/html.go` comments:

```go
/*
Responsibilities
- Normalize malformed markup
- Remove empty or duplicate nodes
- Stabilize heading hierarchy

This stage ensures downstream Markdown conversion is deterministic.
*/
```

Notice what’s *missing*:

* no “improve clarity”
* no “reorganize content”
* no “fix documentation mistakes”

That omission is intentional.

---

### One-sentence summary to keep in mind

> **Task 3.2 fixes the document so it is structurally valid, not so it is better written.**

---

**for Task 3.2, the sanitizer is explicitly allowed to mutate the input HTML**, but **only in very narrow, structural ways**. This is intentional and required by the design.

Here is the precise model we should hold while implementing it.

---

**the sanitizer may mutate the DOM** — but **only to repair provable structural defects**.
It must **never** mutate content in a way that depends on meaning, intent, or preference.

If a safe structural repair is not possible **without guessing**, the sanitizer must **fail**, not “best-effort” clean.

This is mandated by the Task 3.2 invariants .

---

## What “mutation” means in Task 3.2

In this project, *mutation ≠ transformation*.

The sanitizer is a **structural gatekeeper**, not a formatter or editor. Its mutations are limited to enforcing **determinism**.

### ✅ Allowed mutations (structural, mechanical)

These are **required** and expected:

* Fix malformed or unclosed tags
* Normalize invalid nesting (e.g. block inside inline)
* Remove empty containers
* Remove *duplicate* nodes with identical structure + content
* Repair heading *levels* **without reordering nodes**

  * e.g. `h1 → h3 → h2` → `h1 → h2 → h3`
* Collapse DOM ambiguity when **exactly one** reading order can be proven

All of these are explicitly allowed by invariants S1–S5 and H1 .

---

### ❌ Forbidden mutations (semantic or interpretive)

These must **never** happen:

* Renaming headings
* Inventing missing headings
* Choosing between multiple plausible document roots
* Reordering content “to make it nicer”
* Splitting one page into multiple documents
* Merging parallel sections into one narrative
* “Best guess” fixes

If *any* of these would be required → **fail with `ErrCauseStructurallyIncorrect`** .

---

## Why mutation is required at all

Downstream stages (Markdown conversion, normalization) **assume**:

* Exactly one document
* A linearizable reading order
* A stable heading hierarchy
* No ambiguity

The extractor (Task 3.1) answers **“what content belongs to this page?”**
The sanitizer (Task 3.2) answers **“is this content structurally provable?”**

To make that provable, **some mutation is unavoidable**.

---

## Critical mental model

Think of the sanitizer as:

> **A compiler verifier, not a linter**

* A compiler is allowed to normalize ASTs
* A compiler is *not* allowed to guess programmer intent
* If the program is ambiguous → compilation fails

That is exactly the sanitizer’s role here.

---

## Practical implementation guidance

When writing sanitizer logic, ask this question **before every mutation**:

> “Can I prove this change is correct *without knowing what the author meant*?”

* **Yes** → mutation allowed
* **No / maybe / seems reasonable** → **fail**

This aligns exactly with:

* Invariant S2 (No Semantic Inference)
* Invariant E1 (Broken DOM is fatal) 

---

## Relationship to later stages (important)

* **Sanitizer (3.2)**

  * May mutate structure
  * Must reject ambiguity
  * Must not enforce “exactly one H1”

* **Normalizer (later task)**

  * Enforces invariants like “exactly one H1”
  * Operates on already-provable structure

Do **not** move normalization concerns into the sanitizer.

---

## Bottom line

* ✅ **Yes**, the sanitizer mutates HTML
* ✅ Mutation is **required**
* ❌ Mutation must **never** be semantic
* ❌ Ambiguity must **never** be resolved heuristically
* 🛑 When in doubt: **fail loudly**