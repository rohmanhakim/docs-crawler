# Link Context

Here, **`Context` means the *Markdown-structural role* the link plays in the document**, not runtime context, not Go `context.Context`, and not crawl context.

It answers one very specific question:

> **“How does this link exist *in Markdown syntax*, and how must it be rewritten later without re-parsing Markdown?”**

---

## What `Context` is *not*

Let’s eliminate common misinterpretations first.

`Context` does **not** mean:

* crawl depth
* discovery source
* DOM position
* scheduler context
* Go cancellation / deadlines
* semantic meaning of the link

All of those belong to *other layers*.

---

## What `Context` *does* mean

`Context` describes **how the link is embedded in Markdown**, i.e. the *syntactic form* and *rewrite contract*.

Markdown has multiple ways to express links, and they are **not interchangeable** once emitted.

Example:

```md
[API](../api)
![Diagram](diagram.svg)
See [this][ref1]

[ref1]: ../guide
```

All three involve URLs, but rewriting them requires **different handling**.

`Context` captures that difference.

---

## Typical `Context` categories

You don’t need all of these on day one, but this is the conceptual space.

### 1. Inline link

```md
[Text](../api)
```

Characteristics:

* URL is inline
* Rewrite requires editing the inline target
* Text must remain untouched

Example context:

```
InlineLink
```

---

### 2. Inline image

```md
![Alt](diagram.svg)
```

Characteristics:

* Asset resolver must download
* Rewrite points to local asset path
* Still inline, but **asset semantics apply**

Example context:

```
InlineImage
```

---

### 3. Reference-style link (definition)

```md
See [API][api-ref]

[api-ref]: ../api
```

Characteristics:

* The *definition* is rewritten, not the usage
* Multiple usages may point to one definition
* Deduplication rules differ

Example context:

```
ReferenceDefinition
```

---

### 4. Fragment-only link

```md
[Jump](#installation)
```

Characteristics:

* No asset resolution
* No downloading
* Possibly rewritten if heading anchors are normalized

Example context:

```
FragmentOnly
```

---

### 5. External absolute link

```md
[GitHub](https://github.com/...)
```

Characteristics:

* Must *not* be rewritten to local paths
* Must not be downloaded
* Still needs observability

Example context:

```
ExternalLink
```

---

## Why `Context` matters downstream

### Asset resolver needs it

The assets layer must decide:

* Should I download this?
* Should I rewrite it?
* Should I leave it untouched?
* If rewriting, *where* in the Markdown do I apply the change?

Without `Context`, the resolver would have to:

* re-parse Markdown text (forbidden)
* guess intent (forbidden)
* or silently skip cases (bug)

---

### Normalization depends on it

Normalization may:

* rewrite anchors
* enforce stable heading IDs
* validate that image references were resolved

That’s impossible if all you have is “some URL”.

---

## Why this is still mdconvert’s responsibility

Even though mdconvert must not do **policy**, it *must* emit **enough structure** so later stages can act deterministically.

This matches the pipeline contract:

* mdconvert = structural projection
* assets = resolution + I/O
* normalize = invariant enforcement

---

## Minimal mental model

Think of `Context` as answering:

> “If I hand this `LinkRef` to the asset resolver, does it know **exactly what kind of Markdown node** it is dealing with, without looking at raw text?”

If yes → good `Context`.
If no → under-specified.

---

## One-line definition you can keep in code comments

> **`Context` describes the Markdown syntactic role of a link reference, so downstream stages can rewrite or preserve it deterministically without re-parsing Markdown.**

That’s all it needs to mean — nothing more, nothing less.

# Link Kind

*Kind* and *Context* are easy to conflate. They are intentionally **orthogonal**.

---

## One-sentence difference

> **Kind = what the link *is***
> **Context = how the link is *embedded in Markdown syntax***

They answer *different questions* and both are needed.

---

## What **Kind** answers

**Kind classifies the semantic nature of the reference target.**

It is about **what the link points to**, independent of syntax.

Typical `Kind` values:

* `Navigation` – points to another document/page
* `Image` – points to an image asset
* `Anchor` – fragment-only (`#section`)
* `External` – absolute URL outside crawl scope

Example:

```md
![Diagram](diagram.svg)
```

* **Kind**: `Image`
* Because it refers to a binary asset

```md
[API](../api)
```

* **Kind**: `Navigation`
* Because it refers to another document

---

## What **Context** answers

**Context classifies how the link appears in Markdown syntax.**

It is about **how the link is written**, independent of what it points to.

Typical `Context` values:

* `Inline` – `[text](url)`
* `ReferenceDefinition` – `[id]: url`
* `ReferenceUsage` – `[text][id]`
* `InlineImage` – `![alt](url)` (syntactically distinct)
* `Autolink` – `<https://example.com>`

Example:

```md
[API][api-ref]

[api-ref]: ../api
```

* **Kind**: `Navigation`
* **Context**:

  * usage: `ReferenceUsage`
  * definition: `ReferenceDefinition`

---

## Why they must be separate

Because **they vary independently**.

### Same Kind, different Context

All of these are **Kind = Image**:

```md
![Alt](img.png)
![Alt][img-ref]

[img-ref]: img.png
```

But their **Context differs**:

* Inline image
* Reference usage
* Reference definition

Rewriting logic is different in each case.

---

### Same Context, different Kind

All of these are **Context = Inline**:

```md
[Guide](../guide)          // Navigation
[GitHub](https://...)      // External
[Jump](#install)           // Anchor
```

But their **Kind differs**, and so does policy:

* download? no
* rewrite? maybe
* normalize? maybe

---

## What breaks if you merge them

If you collapse Kind and Context into one enum, you will:

* Encode policy into mdconvert (forbidden)
* Create an explosion of combined cases
* Make assets/normalize logic brittle
* Lose the ability to add new syntax forms later

Example of a **bad combined enum**:

```go
InlineExternalLink
InlineInternalLink
ReferenceImage
ReferenceExternalImage
...
```

This is exactly what you want to avoid.

---

## Design rule of thumb

* **Kind** → *asset & policy decisions*
* **Context** → *syntax & rewrite mechanics*

If a downstream decision depends on:

* *what to do with the target* → Kind
* *how to edit Markdown text* → Context

Then you split them correctly.

---

## Minimal viable separation (for Task 4.1)

You don’t need to over-engineer. Even this separation is enough:

```go
type LinkKind int
const (
    KindNavigation
    KindImage
    KindAnchor
    KindExternal
)

type LinkContext int
const (
    ContextInline
    ContextReferenceDef
)
```

You can extend later **without breaking invariants**.

---

## Final mental model (keep this)

> **Kind tells you *what the link means*.
> Context tells you *where and how it lives in Markdown*.**

That distinction is the reason `LinkRef` exists at all.
