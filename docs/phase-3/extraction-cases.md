# Extraction Cases (Task 3.1)

Task 3.1 is about **content extraction correctness**, not full sanitization or normalization. So we only test **selection + fallback**, not deep cleanup or invariants.

---

# Task 3.1 — DECIDED TEST SCOPE

## 🎯 Task 3.1 responsibility (precise)

> **Identify and return the correct main content node, using layered heuristics and fallback, or fail if no meaningful content exists.**

**Out of scope for 3.1:**

* Removing inline chrome inside content
* Heading normalization
* H1 invariants
* Markdown conversion correctness
* Asset handling

---

# ✅ REQUIRED test cases for Task 3.1

These are the **minimum, sufficient, industry-aligned cases** we should implement *now*.

---

## 1. Semantic container — valid (must pass)

### Case 3.1-A: `<main>` with meaningful content

* `<main>` contains:

  * heading + paragraph OR
  * code block
* ✅ Extraction succeeds
* ✅ `<main>` chosen

**Purpose:** golden path

---

## 2. Semantic container — invalid → fallback

### Case 3.1-B: `<main>` exists but empty

```html
<main></main>
```

* ❌ Reject semantic candidate
* ✅ Fallback engaged
* ❌ No error yet

---

### Case 3.1-C: `<main>` contains only navigation

```html
<main>
  <ul><li><a href="/x">X</a></li></ul>
</main>
```

* ❌ Reject semantic candidate
* ✅ Fallback engaged

---

## 3. Secondary container heuristic

### Case 3.1-D: `<main>` invalid, `<article>` valid

```html
<main></main>
<article><h1>Doc</h1><p>Text</p></article>
```

* ❌ Reject `<main>`
* ✅ Accept `<article>`

**Purpose:** layered fallback correctness

---

## 4. No semantic containers → text-density fallback

### Case 3.1-E: No `<main>` / `<article>`

* One large text-heavy `<div>`

* One link-heavy sidebar `<div>`

* ✅ Choose text-heavy div

* ✅ Extraction succeeds

**Purpose:** non-semantic layout support

---

## 5. Code-heavy documentation (important)

### Case 3.1-F: Code-dominant content

```html
<article>
  <pre><code>...</code></pre>
</article>
```

* ✅ Must be considered meaningful
* ❌ Must NOT be rejected as “low prose”

**Purpose:** documentation-specific heuristic

---

## 6. Total failure case (must error)

### Case 3.1-G: No meaningful content anywhere

* Only nav, links, placeholders

* ❌ All heuristics fail

* ✅ Return `ErrCauseNoContent`

**This is the only case where extraction fails in 3.1.**

---

# ❌ EXPLICITLY DEFERRED (NOT Task 3.1)

These **must not** be implemented or tested yet:

| Case                            | Reason              |
| ------------------------------- | ------------------- |
| Inline “Edit on GitHub” removal | Sanitizer concern   |
| Breadcrumb stripping            | Sanitizer concern   |
| Footer/header cleanup           | Sanitizer concern   |
| Multiple H1 validation          | Normalize stage     |
| Inline TOC handling             | Later task          |
| Broken DOM repair               | Sanitizer           |
| Deterministic Markdown          | mdconvert/normalize |

If we handle these now, we'll **blur stage boundaries**.

---

# 🔒 Task 3.1 extractor invariant (final)

By the end of Task 3.1, the extractor guarantees:

> **If `Extract()` returns success, the returned node contains meaningful document content.
> If no heuristic can produce such a node, it fails with `ErrCauseNoContent`.**

Nothing more. Nothing less.

---

## Final decision summary (one paragraph)

For Task 3.1, implement and test **only**: semantic container selection, validation of meaningful content, layered fallback to secondary heuristics, and a single hard failure when all heuristics fail. Chrome stripping, DOM cleanup, and structural enforcement are **explicitly out of scope** and belong to later tasks.
