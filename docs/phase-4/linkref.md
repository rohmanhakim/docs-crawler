# LinkRef

For **Task 4.1 (HTML → Markdown conversion)**, `LinkRef` is **not** a generic URL or anchor helper. It is a **boundary abstraction** that decouples *how links are represented during Markdown conversion* from *how they are later resolved, rewritten, or materialized*.

In short:

> **`LinkRef` is the mdconvert layer’s canonical, lossless representation of a link reference, independent of URL resolution, asset downloading, or storage policy.**

---

## What problem `LinkRef` is solving

At the mdconvert stage, you are in a **pure transformation phase**:

* Input: `SanitizedHTMLDoc`
* Output: `ConversionResult`
* **No I/O**
* **No policy**
* **No filesystem knowledge**
* **No asset downloading**

However, links in Markdown need *future decisions*:

* Is this an internal doc link?
* Is this an image?
* Should it be rewritten to a local path?
* Should it be dropped?
* Does it require downloading?

Those decisions belong to **later stages** (assets, normalize, storage), not mdconvert.

So mdconvert must:

* Preserve *exact link intent*
* Avoid prematurely choosing a concrete URL string
* Remain deterministic and reversible

That’s what `LinkRef` abstracts.

---

## What `LinkRef` represents (conceptually)

A `LinkRef` represents **“a reference to something the document links to”**, not a resolved URL.

It typically needs to carry:

* **Original href/src value** (as found in sanitized HTML)
* **Link kind**

  * navigation link
  * image
  * anchor-only
  * external vs same-origin
* **Context**

  * inline link vs reference-style
  * image vs text link
* **Provenance**

  * where in the document it appeared (for rewrite determinism)

Importantly:

* It is **not yet rewritten**
* It is **not validated**
* It is **not downloaded**

---

## What `LinkRef` must *not* do

`LinkRef` must **not**:

* Resolve relative URLs to absolute
* Touch the filesystem
* Decide whether a link is allowed
* Decide local output paths
* Perform deduplication
* Perform crawling admission

All of that happens **after** mdconvert.

This aligns with the design constraint that:

> *Each stage is isolated and performs a single responsibility* 

---

## Why `ConversionResult` owns `LinkRef`

Notice the current definition:

```go
type ConversionResult struct {
    linkRef []LinkRef
}
```

This signals that:

* The Markdown document is **not just text**
* It is a structured artifact with **deferred references**
* Assets and links are **first-class data**, not string hacks

This is intentional so that:

* `assets.Resolver` can traverse and rewrite links safely
* Markdown text emission can be deterministic
* Asset failures can be attributed precisely (URL, type, context)

This matches the Asset Manager’s responsibility to *rewrite Markdown references* .

---

## How this fits the pipeline contract

| Stage         | Responsibility                            |
| ------------- | ----------------------------------------- |
| sanitizer     | Extract URLs from DOM (for crawling only) |
| **mdconvert** | Emit Markdown + abstract link references  |
| assets        | Resolve, download, rewrite links          |
| normalize     | Enforce structural invariants             |
| storage       | Persist final concrete paths              |

The sanitizer’s discovered URLs (`SanitizedHTMLDoc.discoveredUrls`) are for **crawl expansion**, not Markdown fidelity.

`LinkRef` is for **document correctness**, not crawling.

---

## A useful mental model

Think of `LinkRef` as similar to:

* An AST node for a link
* A symbol reference before linking
* A promise of resolution, not the resolution itself

If mdconvert emitted final URLs directly, you would:

* Collapse pipeline boundaries
* Lose observability
* Make asset failures unrecoverable
* Violate determinism guarantees 

---

## Practical guidance for Task 4.1

When implementing mdconvert:

* Parse `<a>` / `<img>` into Markdown *syntax*
* Store their targets as `LinkRef`, not raw strings
* Treat `LinkRef` as **opaque** outside mdconvert
* Do **no resolution logic** here

If you do that, Task 4.1 stays small, testable, and correct—and assets can evolve independently later.

---

## What `LinkRef` look like 

Not prescribing exact code, but **structurally** it should be closer to:

```go
type LinkRef struct {
    Raw string        // original attribute value from href / src
    Kind LinkKind     // link | image | anchor
    Context LinkCtx   // inline, reference-style, etc.
}
```

Key properties:

* **one LinkRef = one document reference**
* preserves **raw value**
* defers **all resolution**
* carries **enough metadata** for later rewriting

Then `ConversionResult` owns **many** of these:

```go
type ConversionResult struct {
    Links []LinkRef
    // markdown structure...
}
```

---

## Design litmus test (use this)

Ask yourself:

> “Can the asset resolver rewrite this link *without re-parsing Markdown text* and *without guessing*?”

If the answer is “no”, the `LinkRef` is under-specified.

---

## Bottom line

`LinkRef` should represent **one unresolved, contextualized document reference**, not a bag of parsed URLs.

---

### One-sentence summary

**`LinkRef` abstracts “what this document links to” without deciding “where that link ultimately points,” preserving semantic fidelity while deferring policy and I/O to later stages.**


## Extracting Link Ref

> **`LinkRef` is *emitted during Markdown conversion*, as part of building `ConversionResult`, even though the source information originates from sanitized HTML.**

So:

* ❌ Not extracted *after* Markdown as a text-parsing step
* ❌ Not a separate pass over raw HTML
* ✅ Constructed **while converting sanitized HTML → Markdown**

---

## Why this matters architecturally

### 1. mdconvert is a *projection*, not a parser

The mdconvert stage:

* Walks the **sanitized DOM**
* Emits **Markdown structure**
* Emits **structured metadata alongside Markdown**

`LinkRef` is part of that *emission*, not a post-hoc analysis.

If you waited until “after conversion” and then scanned Markdown text:

* You’d be re-parsing Markdown (explicitly disallowed)
* You’d lose determinism
* You’d violate stage isolation

---

## The correct mental model

Think of mdconvert as producing **two coupled outputs at once**:

```
SanitizedHTML
      │
      ▼
┌─────────────────────────┐
│   Markdown Conversion   │
│                         │
│  - Markdown text        │
│  - []LinkRef            │
└─────────────────────────┘
          │
          ▼
     MarkdownDoc
```

So a `LinkRef` is created **at the moment** you decide to emit:

```md
[Text](../api)
```

not later by scanning the string `"../api"`.

---

## Why it cannot be “from raw HTML”

You *do* look at HTML attributes (`href`, `src`) to build it, but:

* HTML structure ≠ Markdown structure
* HTML links don’t tell you:

  * inline vs reference-style Markdown
  * whether you emitted an image or link syntax
  * where the Markdown rewrite point is

Those decisions only exist **after** conversion logic runs.

So the authoritative source of truth is:

* **Markdown emission decision**
* informed by sanitized HTML

---

## Why it cannot be “after Markdown”

If you extracted from Markdown text afterward, you’d have to:

* parse Markdown
* infer syntax roles
* guess rewrite locations

That breaks:

* determinism guarantees
* asset resolver contracts
* your own “no re-parsing” rule

---

## One-line rule you can rely on

> **`LinkRef` is produced by mdconvert *at emit time*, using sanitized HTML as input, and stored alongside the Markdown structure—not discovered later by scanning text.**

If you follow that rule, every downstream stage (assets, normalize, storage) stays clean, deterministic, and policy-free.
