# CSS Reasoning

**The sanitizer must not inspect or reason about CSS at all.**

Longer, precise answer grounded in **Task 3.2 invariants**:

---

## L1 (Layout-dependent order) — What it *actually* requires

Invariant **L1 — Stable Reading Order** says the sanitizer must be able to linearize the document **purely from DOM structure**, without relying on presentation or layout rules .

That implies a **negative obligation**, not a feature requirement:

> If correct reading order depends on CSS → the sanitizer must **fail**, not compensate.

---

## Should we check CSS properties like `flex-direction`, `order`, `grid-template-areas`?

**No — and doing so would be a design violation.**

### Why CSS inspection is forbidden

Checking CSS would imply:

* Inferring reading order from visual layout
* Interpreting author intent (“this column comes first visually”)
* Making viewport- or rendering-dependent assumptions

All of these violate:

* **Invariant S1 (Deterministic Structure)**
* **Invariant S2 (No Semantic Inference)**
* **Invariant L1 (Layout-independent linearization)** 

Once we consult CSS, the structure is **no longer provable from the DOM alone**.

---

## Correct sanitizer behavior for CSS-dependent layouts

| Situation                                                   | What the sanitizer should do    |
| ----------------------------------------------------------- | ------------------------------- |
| DOM order is linear and sensible, CSS just decorates        | ✅ Accept (ignore CSS entirely)  |
| DOM order is meaningless without CSS reordering             | ❌ Fail with `ErrCauseStructurallyIncorrect` |
| Flex/Grid reorder content visually (`order`, `row-reverse`) | ❌ Fail                          |
| Multi-column/grid dashboards with interleaved DOM           | ❌ Fail                          |

We **do not detect CSS** to make this decision.
We detect **DOM ambiguity**.

CSS merely explains *why* ambiguity exists — it must not be used to resolve it.

---

## Practical rule of thumb (implementation-safe)

> **If two reasonable readers could disagree on reading order using only the DOM tree, the sanitizer must reject the document.**

No CSS parsing.
No style attribute inspection.
No layout heuristics.

Just DOM topology and node order.

---

## Concrete examples

### ❌ Must fail (even if CSS makes it look fine)

```html
<div class="grid">
  <div>Main content</div>
  <div>Sidebar</div>
</div>
```

If the DOM order doesn’t reflect a single obvious reading flow and relies on CSS grid placement → **ambiguous**.

### ✅ Must pass

```html
<article>
  <h1>Title</h1>
  <p>Paragraph</p>
  <p>Next paragraph</p>
</article>
```

Linear, DOM-derived, CSS-independent.

---

## Why this matters downstream

Markdown conversion and RAG chunking **assume**:

* A stable, linear document
* No presentation-dependent semantics

Letting CSS influence sanitizer logic would silently inject nondeterminism and violate the crawler’s core design principles .

---

## Final answer (crisp)

**Do not check `grid-template-areas`, `flex-direction`, `order`, or any CSS at all.**
If reading order depends on them, the document is **invalid for Task 3.2** and must fail with `ErrCauseStructurallyIncorrect`.

This is intentional, not a limitation.
