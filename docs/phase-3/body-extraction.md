# Body Extraction Rule

We should **not return `<body>` by default**.
Treat `<body>` as a **last-resort container**, and **prefer a meaningful child container whenever one exists**.

This is an important extractor design decision, and industry practice is very consistent here.

---

## The correct rule (clear and actionable)

> **If `<body>` scores highest only because all chrome was removed, attempt to select the best meaningful child first.
> Return `<body>` only if no child container is more specific and meaningful.**

So the answer is **conditional**, not absolute.

---

## Why `<body>` is special (and dangerous)

`<body>` is:

* A **structural root**, not a content container
* Almost always present
* Often wins on raw text length simply because it contains everything

If we return `<body>` too eagerly:

* We'll blur document boundaries
* We'll pass unnecessary wrapper noise downstream
* We'll make sanitization and Markdown conversion harder
* We'll reduce determinism (future cleanup steps depend on structure)

Industry systems treat `<body>` as a **fallback of last resort**.

---

## Decision matrix (this is the key part)

### Case 1: `<body>` contains a clear content child

```html
<body>
  <div>
    <h1>Title</h1>
    <p>Actual documentation</p>
  </div>
</body>
```

* `<body>` scores highest by size
* Child `<div>` also scores high and is **more specific**

✅ **Return the child `<div>`**, not `<body>`

---

### Case 2: `<body>` contains multiple structural wrappers

```html
<body>
  <div class="layout">
    <div class="content">
      <h1>Doc</h1>
      <p>Text</p>
    </div>
  </div>
</body>
```

* `<body>` wins on size
* `.content` wins on semantic density

✅ **Return `.content`**

---

### Case 3: `<body>` is effectively the document

```html
<body>
  <h1>README</h1>
  <p>Single-page documentation.</p>
  <pre><code>...</code></pre>
</body>
```

* No meaningful grouping elements
* `<body>` children are direct content

✅ **Return `<body>`**

This is valid and common for:

* Minimal docs
* Generated README-like pages
* Very old static HTML

---

## How industry extractors encode this rule

They use **specificity bias**:

> Prefer the **smallest subtree** that:
>
> * Contains the majority of meaningful content
> * Has a score close to the maximum

In practice:

* Compute scores for all candidate nodes
* If `<body>` is the top scorer:

  * Check its children
  * If a child has a score ≥ ~70–80% of `<body>` → choose the child
  * Otherwise → accept `<body>`

No magic — just bias toward specificity.

---

## What *not* to do

❌ Always return `<body>` if it wins
❌ Always reject `<body>` categorically
❌ Add special-case hacks tied to tag names only

Either extreme breaks real pages.

---

## How this fits Task 3.1 cleanly

For **Task 3.1**, the correct behavior is:

* `<body>` **is allowed** as a valid extraction result
* But **only when no more specific meaningful child exists**

This keeps the extractor:

* Robust
* Deterministic
* Correctly layered (selection now, cleanup later)

---

## One-sentence final answer

> **Do not return `<body>` by default; prefer the most specific meaningful child, and only return `<body>` when it truly represents the document content.**
