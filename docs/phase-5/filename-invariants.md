# Filename Invariants

Good.
Let’s formalize this properly so it becomes a **system invariant**, not an implementation detail.

This defines the **Canonical URL → Filename Identity Contract** for Task 5.2.

---

## Canonical URL → Filename Identity Specification

### 1. Purpose

Define a **deterministic, stable, collision-resistant identity mapping**:

```
CanonicalURL  →  Filename
```

This mapping MUST:

* Be deterministic across runs
* Be independent of content
* Be independent of crawl timing
* Survive minor HTML changes
* Produce identical output given identical canonical URLs

---

## 2. Canonical URL Contract (Pre-Hash Invariant)

The URL used for hashing MUST satisfy all of the following:

#### U1 — Scope-Admitted URL

The URL MUST be:

* Post-robots decision
* Post-scope validation
* The exact URL submitted to the frontier

You MUST NOT hash:

* Discovered raw URLs
* Redirect intermediates
* Pre-normalized URLs

---

#### U2 — Canonicalization Rules

Canonicalization MUST enforce:

1. **Lowercase host**
2. **Remove default port**

   * `:80` for http
   * `:443` for https
3. **Strip fragment**

   * Remove `#...`
4. **Strip query parameters**

   * Unless explicitly allowed by config
5. **Normalize path**

   * Remove duplicate slashes
   * Resolve `.` and `..`
6. **Trailing slash normalization**

   * Choose one policy:

     * Always remove trailing slash except root
     * OR always keep trailing slash
   * Must be consistent

If canonicalization is nondeterministic → filename identity is broken.

---

## 3. Hashing Contract

### H1 — Algorithm

MUST use:

```
SHA-256 / BLAKE3
```

Rationale:

* Collision resistance
* Stable across platforms
* Widely supported
* Future-proof

---

### H2 — Input Encoding

Hash input MUST be:

```
canonicalURL.String()
```

Encoded as UTF-8 bytes.

No additional whitespace.
No metadata.
No timestamp.
No depth.

Only canonical URL string.

---

### H3 — Truncation

Full 64 hex chars are unnecessary.

Recommended:

```
First 12–16 hex characters
```

Example:

```
a93f2d91c4b1.md
```

Collision probability with 16 hex chars (64 bits) is negligible for documentation scale.

---

## 4. Filename Construction

Final filename MUST be:

```
<url_hash>.md
```

No title.
No depth.
No section.
No slug.

Directory layout may still organize by section if desired, but filename identity must remain URL-hash-based.

---

## 5. Determinism Invariants

The following MUST hold:

#### D1 — Same canonical URL → Same filename

#### D2 — Different canonical URLs → Different filename (with overwhelming probability)

#### D3 — HTML content changes → Same filename

#### D4 — Re-crawl with same input → Identical output filename

#### D5 — Hash function change → BREAKING CHANGE (versioned)

If you ever change canonicalization or hashing, that is a version bump of the crawler.

---

## 6. Explicit Non-Goals

Filename identity MUST NOT depend on:

* Markdown content
* Content hash
* H1 text
* Crawl depth
* Section name
* Discovery source
* Crawl timestamp

Those belong in metadata.

---

## 7. Where This Lives

This logic MUST live in:

```
storage.Sink
```

Because:

* Filename generation is persistence logic
* Identity mapping belongs at the storage boundary
* Other pipeline stages must remain unaware of filenames

This respects separation defined in your technical design .

---

## 8. Optional: Sidecar Metadata (Recommended)

Store alongside Markdown:

```json
{
  "url_hash": "...",
  "content_hash": "...",
  "source_url": "...",
  "crawl_depth": 2
}
```

Important distinction:

* `url_hash` → identity
* `content_hash` → change detection

Never mix them.

---

## 9. Formal Identity Invariant

You may codify this in documentation as:

> Document identity is defined exclusively as the SHA-256 hash of the canonicalized source URL.
> This identity is immutable across reruns and independent of document content.

This becomes a **storage-level invariant**.

---

## 10. Edge Case: Redirect Final URL

Important decision:

If:

```
A → 301 → B
```

Which do you hash?

Correct answer:

* Hash the **final canonical URL (B)**
* The scheduler must treat that as the identity URL

Otherwise you risk duplicate content with different filenames.

---

## Final Verdict

You now have a:

* Deterministic identity layer
* Collision-resistant filename mapping
* Content-stable storage model
* RAG-safe persistence scheme
* Clean architectural separation

This is the correct design for Task 5.2.