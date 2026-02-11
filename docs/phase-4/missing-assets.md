# Missing Assets Resolution

## Canonical rule (non-negotiable)

> **If an asset fails to download or fails to be written, the Markdown MUST remain unchanged for that asset reference.**

In other words:

* **Do not rewrite the URL**
* **Do not delete the reference**
* **Do not replace it with a placeholder**
* **Do not guess a local path**

Only **successfully resolved assets** may be rewritten.

This follows directly from the pipeline’s determinism and non-invention principles .

---

## Why this rule exists

### 1. Provenance preservation

The original Markdown reference still correctly expresses *author intent*. Rewriting to a nonexistent local path would silently corrupt meaning.

### 2. Determinism & auditability

A failed asset is an **observable failure**, not a transformation decision. It must surface via metadata, not content mutation.

### 3. Separation of concerns

* `assets.Resolver` → *attempt resolution*
* `metadata` → *record failure*
* `storage` → *persist what is valid*

No stage is allowed to “patch over” missing artifacts.

---

## Correct behavior matrix

| Scenario                    | Markdown rewrite?       | Metadata         | Error severity |
| --------------------------- | ----------------------- | ---------------- | -------------- |
| Image downloaded & written  | ✅ rewrite to local path | `RecordArtifact` | —              |
| Download fails (network)    | ❌ no rewrite            | `RecordError`    | Recoverable    |
| Write fails (disk, perms)   | ❌ no rewrite            | `RecordError`    | Usually Fatal  |
| Partial asset batch failure | Mixed (per-asset)       | Per-asset errors | Mixed          |

This is fully consistent with the **Asset Policies** and **Failure Modes** sections of the design .

---

## What the returned document should look like

Your `AssetfulMarkdownDoc` must therefore support **partial resolution**:

```text
![ok](../assets/images/ok.png)
![failed](https://original.site/broken.png)
```

Both references coexist. This is correct.

Downstream:

* `normalize` does **nothing** about this
* `storage` writes the Markdown as-is
* Consumers (humans or RAG) can still see the image intent

---

## Metadata expectations (important)

Every failed asset attempt **must** emit metadata, for example:

* `AttrAssetURL = original URL`
* error cause:

  * `CauseNetworkFailure` (download)
  * `CauseStorageFailure` (write)

This is already aligned with your `AssetsError → metadata.ErrorCause` mapping logic .

> **Key invariant:** metadata is observational only — it must never influence rewriting decisions.

---

## What you must NOT do (common traps)

❌ Rewrite to a guessed local filename
❌ Replace with empty string or alt-text only
❌ Insert HTML comments like `<!-- failed -->`
❌ Fail the whole document for a single missing image

Those all violate either determinism, semantic fidelity, or single-responsibility rules.

---

## Final takeaway

* **Successful asset → rewrite**
* **Failed asset → leave original URL untouched**
* **Always log the failure**
* **Never invent or “fix up” content**

If you follow this rule, your asset resolver remains:

* deterministic
* semantically faithful
* debuggable
* future-proof

This is exactly what task 4.2 is meant to enforce.
