# Sanitizer Example Test Pages

**~10–12 carefully designed HTML samples are sufficient** to cover *all* sanitizer invariants in `sanitizer-invariants.md`. More samples won’t increase correctness unless they introduce *new structural ambiguity classes*.

Below is the **minimal, invariant-driven test set**, mapped explicitly to the invariants in **Task 3.2** .

---

## Why the number is small

The sanitizer invariants are **structural**, not combinatorial:

* They classify **types of document ambiguity**
* They are **binary-gated** (provable structure → accept, otherwise → fail)
* Many HTML variations collapse into the *same invariant class*

So we test **equivalence classes**, not permutations.

---

## Minimal Canonical Test Matrix

### ✅ Passing cases (sanitizer MUST succeed)

### 1. Clean, canonical document

**Covers:** S1, S3, L1

* Single `<main>`
* One `h1`, ordered headings
* Linear DOM

---

### 2. Malformed but structurally unambiguous HTML

**Covers:** S4

* Unclosed tags
* Invalid nesting (`<p><div>`)
* Still one clear document

➡ proves *structural repair without semantic inference*

---

### 3. Skipped heading levels (repairable)

**Covers:** H1

```html
<h1>Title</h1>
<h3>Subsection</h3>
<h2>Section</h2>
```

➡ sanitizer may renumber, **must not reorder**

---

### 4. Duplicate nodes (exact structural duplicates)

**Covers:** S4

* Same subtree repeated twice
* Identical content + structure

➡ sanitizer may deduplicate

---

### 5. No `<h1>`, but provable anchors

**Covers:** H3 (positive case)

* `<article>` + `<section>` + `<h2>` hierarchy
* Still one provable structure

---

## ❌ Failing cases (sanitizer MUST error)

### 6. Multiple `h1` with no dominant root

**Covers:** H2, S3

* Parallel top-level sections
* Typical “hub” or comparison page

➡ **must fail**, even if downstream could “handle it”

---

### 7. Two competing document roots

**Covers:** S1, S3

* Two `<article>` siblings
* Both plausible roots

➡ ambiguity → fail

---

### 8. Layout-dependent reading order

**Covers:** L1

* DOM order differs from visual order
* CSS grid / columns implied

➡ sanitizer must not guess reading order

---

### 9. Structure requires semantic inference

**Covers:** S2

* Headings missing, content implies sections only by wording
* E.g. bold text used as pseudo-headings

➡ forbidden to infer → fail

---

### 10. Page that *should* be split (but sanitizer must not)

**Covers:** S5

* Clearly two documents on one page
* e.g. two manuals concatenated

➡ sanitizer must **fail**, not split

---

### 11. Completely broken / ambiguous DOM

**Covers:** E1

* Tag soup
* Multiple valid parses
* No stable hierarchy

➡ `ErrCauseStructurallyIncorrect`

---

### 12. Unparseable HTML (optional but recommended)

**Covers:** E2

* HTML cannot be parsed into a DOM at all

➡ distinct from ambiguity

---

## Summary Table

| Category      | Count     |
| ------------- | --------- |
| Passing cases | 5         |
| Failing cases | 6–7       |
| **Total**     | **10–12** |

---

## Design takeaway (important)

If we feel we need **20+ samples**, it usually means:

* We are testing *implementation behavior*, not *invariants*
* Or we are allowing sanitizer heuristics that violate **S2 / H2**

---

# Page Names

Below is a **canonical, invariant-aligned fixture naming scheme** for **Task 3.2 (sanitizer)**.
The names are designed to be:

* **Invariant-first** (we can see *why* it exists from the name)
* **Binary** (either must-pass or must-fail)
* **Stable** as the codebase grows

This mirrors how we should reason about sanitizer correctness.

---

## Directory Layout

```
internal/sanitizer/fixture
├── pass/
├── fail/
└── expected/
```

---

## ✅ `pass/` — sanitizer MUST succeed

### 1. Canonical document

```
pass/
└── s1_single_root_linear.html
```

**Invariants:** S1, S3, L1
Single document, single root, linear DOM.

---

### 2. Malformed but repairable structure

```
pass/
└── s4_repairable_malformed_dom.html
```

**Invariants:** S4
Broken tags, invalid nesting, but unambiguous structure.

---

### 3. Repairable heading hierarchy

```
pass/
└── h1_repairable_heading_skips.html
```

**Invariants:** H1
Skipped heading levels; renumbering allowed without reordering.

---

### 4. Duplicate structural nodes

```
pass/
└── s4_duplicate_nodes_identical.html
```

**Invariants:** S4
Exact subtree duplication; safe to remove.

---

### 5. No h1 but provable structure

```
pass/
└── h3_structural_anchors_without_h1.html
```

**Invariants:** H3 (positive case)
Document structure is provable without inventing headings.

---

## ❌ `fail/` — sanitizer MUST error

### 6. Multiple h1 without primary root

```
fail/
└── h2_multiple_h1_ambiguous_root.html
```

**Invariants:** H2, S3
Parallel top-level sections.

---

### 7. Competing document roots

```
fail/
└── s3_competing_document_roots.html
```

**Invariants:** S1, S3
Two plausible top-level containers.

---

### 8. Layout-dependent reading order

```
fail/
└── l1_layout_dependent_order.html
```

**Invariants:** L1
Reading order depends on CSS or visual layout.

---

### 9. Semantic inference required

```
fail/
└── s2_semantic_inference_required.html
```

**Invariants:** S2
Structure implied only by wording or styling.

---

### 10. Page that implies multiple documents

```
fail/
└── s5_implied_multiple_documents.html
```

**Invariants:** S5
Would require splitting or synthesis.

---

### 11. Structurally ambiguous DOM

```
fail/
└── e1_structurally_ambiguous_dom.html
```

**Invariants:** E1
Multiple valid interpretations.

---

### 12. Unparseable HTML (optional)

```
fail/
└── e2_unparseable_html.html
```

**Invariants:** E2
DOM cannot be constructed at all.

---

## `expected/` — if the sanitizer have to repair, this directory is where the fixture for the expected result be placed
---

## Naming Rules (use these consistently)

```
<invariant>_<concise_description>.html
```

* Prefix with **the invariant that justifies existence**
* Use **singular reason** per file
* Never encode expected behavior like `should_fail`
* Never reference implementation terms

---

## Why this matters

When a test fails, we should be able to say:

> “Invariant **H2** is broken”

—not—

> “The sanitizer didn’t like this page.”

This fixture set becomes **executable documentation** for Task 3.2.
