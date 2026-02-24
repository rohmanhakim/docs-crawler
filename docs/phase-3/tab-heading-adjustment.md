# Tab Heading Adjustment Specification (THAS)

## 1. Scope

This specification defines the required heading-level transformations applied to tab panel content after:

* Tab container detection
* Deterministic linearization
* Promotion of tab labels into heading nodes

It applies only to heading-level normalization inside tab panels.

It does NOT apply to:

* Global document H1 enforcement (normalize stage)
* Non-tab structural repairs
* RAG chunking policies

---

## 2. Definitions

### 2.1 Inserted Tab Heading

A heading node inserted by the sanitizer to represent a tab label.

Let:

* `L` = heading level of the inserted tab heading (1–6)

Example:

```
H2 Python
```

Then `L = 2`.

---

### 2.2 Panel Heading Set

All heading nodes (`H1`–`H6`) that are descendants of the tab panel subtree.

Let:

* `Hmin` = minimum heading level within that panel

---

## 3. Core Invariant

After linearization:

> No heading inside a tab panel may have level ≤ L.

This guarantees:

* Valid hierarchical nesting
* Deterministic tree structure
* Compatibility with downstream Markdown conversion 

---

## 4. Adjustment Rule

### Rule TH1 — Uniform Downward Shift

If:

```
Hmin ≤ L
```

Then sanitizer MUST apply a uniform downward shift:

```
Δ = (L + 1) − Hmin
```

For every heading `Hi` in the panel:

```
NewLevel = min(Hi + Δ, 6)
```

Where:

* `6` is the maximum heading level
* Relative differences between headings MUST be preserved
* DOM order MUST NOT change

---

## 5. Structural Guarantees

After transformation, the following MUST hold:

### G1 — Proper Nesting

All panel headings satisfy:

```
NewLevel > L
```

---

### G2 — Relative Depth Preservation

For any two headings `A` and `B` in the same panel:

```
(A.level − B.level) before shift
==
(A.level − B.level) after shift
```

Unless clamped by H6 ceiling.

---

### G3 — Order Preservation

* No heading nodes may be reordered
* No subtree may be rearranged
* Only numeric level modification is allowed

This is explicitly permitted structural repair .

---

## 6. H6 Ceiling Rule

### Rule TH2 — Maximum Depth Clamp

If:

```
Hi + Δ > 6
```

Sanitizer MUST clamp:

```
NewLevel = 6
```

This is a mechanical Markdown constraint.

Clamping does NOT constitute semantic inference.

---

## 7. Failure Conditions

Sanitizer MUST return `ErrCauseBrokenDOM`  if:

### F1 — Competing Roots Within Panel

The panel contains multiple `H1` elements representing independent document roots.

### F2 — Heading Repair Requires Reordering

If preserving hierarchy requires moving nodes rather than numeric adjustment.

### F3 — Non-Uniform Repair Required

If adjustment cannot be expressed as a single uniform shift `Δ` applied to all headings.

Example (must fail):

```
H2
  H4
H3
```

If valid structure requires selective repair beyond uniform shift.

---

## 8. Non-Permitted Behaviors

Sanitizer MUST NOT:

* Rename headings
* Merge heading levels
* Insert artificial intermediate headings
* Infer intended hierarchy
* Delete conflicting headings

All such behaviors violate the “No Semantic Inference” invariant .

---

## 9. Special Cases

### Case A — No Headings in Panel

No adjustment required.

Panel content remains under inserted tab heading.

---

### Case B — Hmin > L

No shift required.

Structure already valid.

---

### Case C — Inserted Heading at H6

If `L = 6` and panel contains headings:

Adjustment is impossible (no deeper level exists).

Sanitizer MUST fail with `ErrCauseBrokenDOM`.

Because:

* Deterministic structure cannot be guaranteed.
* Markdown depth limit prevents valid nesting.

---

## 10. Interaction With Downstream Stages

After THAS is applied:

* Markdown conversion assumes hierarchy is valid 
* Normalize enforces global H1 invariant only
* No downstream stage may revisit tab-specific adjustments

Tab adjustments are sanitizer-exclusive.

---

## 11. Determinism Guarantee

Given identical input DOM:

* Tab heading insertion level `L` is deterministic
* Computed `Δ` is deterministic
* Resulting heading levels are deterministic
* No CSS, JS, or viewport state influences behavior

This satisfies:

> Deterministic Structure Invariant 

---

# Final Non-Negotiable Principle

Tab heading adjustment is:

* A mechanical numeric transformation
* Applied uniformly
* Order-preserving
* Semantics-neutral

If numeric transformation alone cannot resolve the conflict:

> The sanitizer MUST reject the document rather than guess intent. 
