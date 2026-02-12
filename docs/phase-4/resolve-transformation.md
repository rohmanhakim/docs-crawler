# Resolve Markdown Transformation

## The intended contract of `assets.Resolver`

In **task 4.2**, the `Resolver` is a **pure transformation stage** in the pipeline:

```
MarkdownDoc
   ↓ (assets.Resolver)
AssetfulMarkdownDoc
```

It **does not invent a new document**, nor does it re-run Markdown conversion. Instead, it:

1. **Takes the Markdown produced by `mdconvert`**
2. **Finds asset references inside it** (images, downloadable files, etc.)
3. **Downloads / deduplicates those assets**
4. **Rewrites the URLs in the Markdown to local paths**
5. **Returns a new document value that embeds the rewritten Markdown**

So the mental model is:

> **Input Markdown → same Markdown content, but with asset URLs rewritten to local paths, plus asset bookkeeping**

This is consistent with the design intent in both the architecture and technical documents .

---

## Important nuance: not “in-place mutation”

Although it *sounds* like “replace URLs in the Markdown,” the design should be understood as:

* **Functionally immutable**
* **Structurally additive**

You should **not** think of `Resolver` as mutating a string buffer in-place.

Instead:

* `mdconvert.MarkdownDoc` is an **asset-agnostic representation**
* `assets.AssetfulMarkdownDoc` is a **strict superset**:

  * rewritten Markdown
  * resolved asset paths
  * hashes / IDs (eventually)
  * metadata needed by storage

Your current type skeleton already hints at this separation:

```go
func (r *Resolver) Resolve(
    markdownDoc mdconvert.MarkdownDoc,
) (AssetfulMarkdownDoc, internal.ClassifiedError)
```

That boundary is correct.

---

## What *must* change vs what must *not*

### ✅ Resolver **does**

* Rewrite **asset URLs only** (e.g. `![img](https://site/foo.png)` → `![img](../assets/images/foo.png)`)
* Download and deduplicate assets
* Emit metadata (`RecordError`, later `RecordArtifact`)
* Preserve Markdown semantics

### ❌ Resolver **does not**

* Change headings, links-to-pages, or structure
* Decide storage layout (that’s `storage.Sink`)
* Normalize Markdown invariants (that’s `normalize`)
* Re-run HTML parsing or Markdown conversion

This division matches the pipeline responsibilities defined in the design docs .

---

## Why returning a *new* document matters

Returning a new `AssetfulMarkdownDoc` instead of “the same Markdown with edits” gives you:

* Determinism (same input → same output)
* Testability (golden files for asset resolution)
* Clear blame boundaries (resolver bugs don’t contaminate mdconvert)
* Future extensibility (e.g. asset manifests, integrity hashes)

It also aligns with your system-wide rule:

> *Each stage is isolated and testable. No stage performs multiple responsibilities.* 

---

## Practical guidance for your implementation

A good internal shape (not API-visible yet) is:

```go
type AssetfulMarkdownDoc struct {
    Markdown string              // rewritten markdown
    Assets   []ResolvedAsset     // bookkeeping
}
```

Where:

* `Markdown` starts life as the input Markdown, with **only asset URLs rewritten**
* Page links (`[link](../other-page.md)`) remain untouched

---

## Bottom line

✔ The resolver conceptually returns the same Markdown content with URLs rewritten
✔ **But** it must do so as a **new document value**, not an in-place mutation
✔ **And** its scope is strictly **assets**, not general link rewriting or normalization

Your current pipeline ordering and type boundaries are **correct** for task 4.2.
