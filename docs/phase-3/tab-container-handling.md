# Tab Container Handling Specification (TCHS)

## 1. Scope

This specification defines how the **Sanitizer (Task 3.2)** must handle DOM structures representing tabbed content containers.

This specification applies only to:

* Multiple sibling content panels
* Representing alternate variants of the same logical section
* Where visibility is controlled by UI state (CSS / JS)

This specification does not apply to:

* Navigation tabs linking to separate pages
* Version switchers that load different documents
* Independent documents embedded in the same page

---

## 2. Definitions

### 2.1 Tab Container

A DOM subtree is considered a *Tab Container* if:

* It contains a finite set of sibling panels
* Exactly one panel is visually active at a time
* All panels are present in the static DOM
* UI state (CSS classes, `aria-selected`, etc.) controls visibility

Example structural pattern:

```html
<div class="tabs">
  <div role="tablist">...</div>
  <div role="tabpanel">...</div>
  <div role="tabpanel">...</div>
  ...
</div>
```

---

### 2.2 Tab Panel

A *Tab Panel* is:

* A content subtree representing one variant
* Not dynamically fetched
* Not conditionally injected by runtime JavaScript
* Structurally self-contained

---

## 3. Core Principle

> Tab containers represent **parallel variants**, not multiple documents.

The sanitizer MUST treat tab containers as **single-document structures** containing multiple deterministic variants.

---

## 4. Structural Requirements

### T1 — All Panels Exist in DOM

If any panel requires JavaScript execution to materialize:

* Sanitizer MUST ignore dynamic execution
* Only static DOM is considered

---

### T2 — Single Document Root

The combined tab container MUST still satisfy:

* Single deterministic document root
* One provable structural hierarchy
* No competing top-level roots

If this cannot be proven → sanitizer MUST fail with `ErrCauseBrokenDOM` 

---

### T3 — Deterministic Linearization

Sanitizer MUST:

1. Preserve all tab panels
2. Preserve DOM order of panels
3. Linearize panels sequentially
4. Insert structural anchors to maintain determinism

Visibility state MUST NOT influence ordering.

---

## 5. Required Transformation

### 5.1 Canonical Linearization

Given:

```html
<div class="tab-container">
  <div id="panel-a">A</div>
  <div id="panel-b">B</div>
  <div id="panel-c">C</div>
</div>
```

Sanitizer MUST transform into:

```
[Panel A Anchor]
Content A

[Panel B Anchor]
Content B

[Panel C Anchor]
Content C
```

Anchors MUST:

* Preserve original label text (if present)
* Not invent meaning
* Not reorder content
* Not merge content

---

### 5.2 Anchor Rules

If tab labels exist (e.g., Python, Java):

* Sanitizer MAY convert labels into subheadings
* Heading level adjustment MUST follow heading repair rules
* Relative order MUST be preserved

If no labels exist:

* Sanitizer MUST NOT invent labels
* It MAY use structural wrappers without semantic text

---

## 6. Prohibited Behaviors

Sanitizer MUST NOT:

* Choose a “primary” tab
* Drop non-visible tabs
* Reorder tabs
* Merge panels based on similarity
* Split panels into separate documents
* Infer intent based on CSS classes

These violate:

* No semantic inference 
* No document synthesis 

---

## 7. Failure Conditions

Sanitizer MUST return `ErrCauseBrokenDOM` if:

### F1 — Multiple Competing Roots

Each panel contains independent `h1` roots with no provable hierarchy.

### F2 — Independent Documents

Panels represent fully independent documentation trees.

Example:

* Tab 1: Complete SDK reference
* Tab 2: Complete API reference

This violates Single Document Root invariant 

---

### F3 — Ambiguous Ordering

If structural analysis cannot determine deterministic order without CSS layout interpretation.

---

## 8. Interaction with Downstream Stages

### Markdown Conversion

* Receives already-linearized structure
* Performs no special tab logic

### Normalization

* Enforces H1 invariant
* Does not reason about tabs

Tab logic MUST NOT leak past sanitizer.

---

## 9. Configuration Extensions (Optional Policy Layer)

This is NOT default behavior.

A policy layer MAY introduce:

```yaml
preferredVariant: "Kotlin"
```

If configured:

* Sanitizer MAY drop non-matching panels
* This decision MUST be deterministic
* Must be documented in metadata
* Must not violate structural invariants

Without explicit configuration:
All panels MUST be preserved.

---

## 10. Rationale

This design satisfies:

| Constraint                | Satisfied |
| ------------------------- | --------- |
| Determinism               | Yes       |
| No semantic inference     | Yes       |
| Stable output across runs | Yes       |
| Single document guarantee | Yes       |
| No UI-state dependence    | Yes       |

It aligns with:

* Sanitizer invariants 
* Technical design principles 
* Content ingestion philosophy 

---

# Final Non-Negotiable Rule

> A tab container is not a UI problem.
> It is a structural linearization problem.

The sanitizer’s job is not to simulate a browser.
It is to produce a provably deterministic document.

Anything else is a design violation.
