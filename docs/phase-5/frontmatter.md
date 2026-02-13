# Frontmatter

For **Task 5.1 (Markdown Normalization → Frontmatter injection)** you are no longer doing structure repair — that was Task 3.2 (Sanitizer) .

At this stage you are shaping documents for:

* Determinism
* RAG ingestion
* Reproducibility
* Traceability
* Auditability

Your own design doc already defines the minimal required fields  and the technical doc reiterates them . But for a serious RAG pipeline, the minimal set is insufficient.

Below is a **recommended frontmatter schema**, split into tiers.

---

## 1️⃣ Minimal (Spec-Compliant Baseline)

This matches your documented requirement :

```yaml
---
title: "Getting Started"
source_url: "https://docs.example.com/guide/getting-started"
section: "guide"
depth: 2
---
```

These are mandatory.

#### Fields

| Field        | Why                                  |
| ------------ | ------------------------------------ |
| `title`      | Required for indexing + display      |
| `source_url` | Canonical provenance                 |
| `section`    | Logical grouping                     |
| `depth`      | Crawl depth (debug + quality signal) |

This is the bare minimum.

---

## 2️⃣ Recommended (Production RAG-Ready)

For a serious retrieval system, I recommend:

```yaml
---
title: "Getting Started"
source_url: "https://docs.example.com/guide/getting-started"
canonical_url: "https://docs.example.com/guide/getting-started"
crawl_depth: 2
section: "guide"
doc_id: "a9f3c2d1"
content_hash: "sha256:ab23..."
fetched_at: "2026-02-12T10:15:00Z"
crawler_version: "v0.1.0"
---
```

#### Why each matters

##### `canonical_url`

After redirects and normalization, this should reflect the resolved URL.
Critical for deduplication.

##### `doc_id`

Deterministic ID (hash of canonical URL).
Useful for vector store keying.

##### `content_hash`

Hash of final normalized Markdown.
Enables incremental recrawl + change detection.

##### `fetched_at`

Observability only.
Do NOT use in control flow (respect metadata design).

##### `crawler_version`

Ensures corpus reproducibility when logic changes.

---

## 3️⃣ Advanced (If You Want High-End RAG Quality)

Only add these if you truly need them:

```yaml
---
language: "en"
content_type: "documentation"
estimated_tokens: 842
has_code_blocks: true
has_tables: false
---
```

These improve:

* Embedding routing
* Chunking heuristics
* Query filtering

But they are optional.

---

## 4️⃣ What You Should NOT Include

Do NOT include:

* retry counts
* HTTP status
* raw metadata events
* robots decisions
* internal pipeline state
* asset URLs

Those belong in metadata recorder, not frontmatter.

Frontmatter = document-level identity & provenance.
Metadata = crawl observability.

You already separated those concerns well.

---

## 5️⃣ Structural Guidance for Your `FrontMatter` Type

Right now you have:

```go
type FrontMatter struct{}
```

I would recommend:

```go
type FrontMatter struct {
    Title         string
    SourceURL     string
    CanonicalURL  string
    CrawlDepth    int
    Section       string
    DocID         string
    ContentHash   string
    FetchedAt     time.Time
    CrawlerVersion string
}
```

Important:

* Fields must be deterministic except `FetchedAt`
* `DocID` should be derived from canonical URL
* `ContentHash` should be computed after normalization
* Section must be derivable from URL path or config, not guessed semantically

---

## 6️⃣ Strict Design Rule (Aligned With Your Architecture)

Frontmatter must:

* Be derivable from deterministic inputs
* Not require semantic inference
* Not depend on external systems
* Be stable across reruns with same input

If two runs produce different frontmatter with identical HTML → design flaw.

---

## 7️⃣ Final Recommendation

For Task 5.1 in your current architecture:

Use this as your production baseline:

```yaml
---
title: ""
source_url: ""
canonical_url: ""
crawl_depth: 0
section: ""
doc_id: ""
content_hash: ""
crawler_version: ""
---
```

Add `fetched_at` only if you explicitly accept non-deterministic output.

---

## Content Hash

### Definition

A **content hash** is a cryptographic digest computed from the **final, normalized Markdown content** of a document.

It is a deterministic fingerprint.

If two documents have identical content → they produce the same hash.
If even one byte differs → the hash changes.

---

### Why It Exists in Your Crawler

In your architecture, it enables:

1. **Change detection**
2. **Incremental recrawling**
3. **Deduplication**
4. **Vector index invalidation**
5. **Reproducibility guarantees**

It is not metadata about the crawl.

It is identity of the content.

---

### Where It Should Be Computed

In your pipeline:

```
Fetch
→ Extract
→ Sanitize
→ Convert
→ Resolve Assets
→ Normalize   ← compute hash here
→ Storage
```

You compute the hash from:

> The fully normalized Markdown string (after frontmatter injection decision, but before writing).

Important:

Do NOT hash:

* Raw HTML
* Extracted DOM
* Pre-sanitized content
* Pre-normalized Markdown

Hash only the **final canonical representation**.

---

### What Algorithm To Use

Use a stable cryptographic hash:

* `SHA-256` (recommended)
* Or `BLAKE3` (faster, modern)

In Go:

```go
import "crypto/sha256"

hash := sha256.Sum256([]byte(markdownString))
```

Store as:

```
sha256:ab23c9...
```

Include the algorithm prefix so it’s future-proof.

---

### What Exactly Gets Hashed?

You must define this strictly to preserve determinism.

Recommended:

```
<normalized_markdown_body>
```

NOT:

* Not including crawl time
* Not including fetched_at
* Not including version
* Not including file path
* Not including asset binary data

Only the document content.

If frontmatter is deterministic and part of the file, you have two options:

#### Option A (Recommended)

Hash only the body, not frontmatter.

This allows metadata changes without content hash change.

#### Option B

Hash entire file including frontmatter.

This makes metadata part of identity.

I recommend Option A.

---

### Why This Matters For RAG

Imagine:

* You crawled 5,000 docs
* You embedded them
* Next week, 2 changed

With content hashes:

* Compare old hash vs new hash
* Only re-embed changed docs
* Keep embeddings stable

Without content hashes:
You must re-embed everything.

---

### Example

Markdown:

````
## Getting Started

Install the CLI:

```bash
npm install mytool
```

````

SHA-256 might become:

```

sha256:9b8d5c6e3a1f...

```

If someone changes:

```

npm install mytool@latest

```

The hash changes completely.

That’s the avalanche property of cryptographic hashes.

---

### Critical Design Constraint

Your content hash must be:

- Deterministic
- Stable across runs
- Based on normalized output
- Independent of crawl timing

If identical HTML input produces different content hashes → normalization is not deterministic.

---

### Relationship To DocID

You should not confuse:

| Field | Meaning |
|--------|---------|
| `doc_id` | Identity of URL |
| `content_hash` | Identity of content |

Example:

Same URL, new content:
- doc_id stays same
- content_hash changes

That is correct behavior.

---

### Summary

Content hash =

> Cryptographic fingerprint of the final normalized Markdown body, used to detect content change and guarantee reproducibility.

---

## DocID

### Definition

`DocID` is the **stable, deterministic identity of a document** independent of its content.

It answers:

> “Which logical document is this?”

Not:

> “What does this document currently contain?”

That distinction is critical.

---

### Conceptual Difference

| Field          | Represents                                            |
| -------------- | ----------------------------------------------------- |
| `doc_id`       | Document identity (stable across content changes)     |
| `content_hash` | Document content state (changes when content changes) |

---

### In Your Crawler Architecture

Given your deterministic, scheduler-controlled pipeline , `DocID` should:

* Be derived from canonical URL
* Be deterministic
* Be stable across recrawls
* Not depend on content
* Not depend on time
* Not depend on crawl depth

It is a **primary key for the document**.

---

### What Should It Be Derived From?

The safest definition:

```
DocID = hash(canonical_url)
```

Where:

* URL is normalized
* Query parameters stripped (unless allowed)
* Fragment removed
* Host canonicalized
* Redirect resolved

So:

```
https://docs.example.com/guide/getting-started
```

→ SHA256
→ `a9f3c2d1...`

---

### Why Not Just Use URL As ID?

Because:

* URLs can be long
* URLs may need normalization
* Some storage systems prefer fixed-length keys
* Vector stores benefit from stable fixed IDs

But logically, yes — URL is the identity.
`DocID` is just a compact, stable encoding of it.

---

### Why You Need It (Practical Reasons)

#### 1️⃣ Incremental Recrawl

Same URL:

* doc_id stays same
* content_hash may change

This lets you:

* Update embeddings in place
* Replace previous vector entry

---

#### 2️⃣ Vector Store Key

Instead of:

```
"source_url": "https://docs.example.com/guide/getting-started"
```

You use:

```
"id": "a9f3c2d1"
```

Much cleaner.

---

#### 3️⃣ Deterministic Storage Mapping

You can name files:

```
docs/a9f3c2d1.md
```

Instead of relying on path mapping.

---

### What It Must NOT Depend On

`DocID` must NOT include:

* crawl_depth
* fetch timestamp
* content
* hash of content
* crawler version
* section

Those are properties.
DocID is identity.

---

### Where It Belongs In Your Pipeline

You should compute it during normalization:

```
Normalize(...)
  → derive canonical_url
  → compute doc_id
  → compute content_hash
  → inject frontmatter
```

Not in storage.
Not in metadata.
Not in fetcher.

Normalization is where identity is finalized.

---

### Example

Input URL:

```
https://docs.example.com/guide/getting-started#install
```

Canonicalized to:

```
https://docs.example.com/guide/getting-started
```

DocID:

```
sha256("https://docs.example.com/guide/getting-started")
→ 1f8c9b2e...
```

Content changes later:

* DocID stays `1f8c9b2e`
* content_hash changes

Correct behavior.

---

### Short Formal Definition

> `DocID` is the deterministic, canonicalized URL-derived identifier that uniquely represents a logical documentation page across crawl executions.

---

In your frontmatter, `Section string` should represent:

> The logical documentation grouping that the page belongs to — derived deterministically from URL structure or crawl configuration.

It is **not semantic classification**.
It is **not inferred topic**.
It is **not extracted from headings**.

It must be mechanically derivable.

---

## Section

### What Section Is (In Your Crawler)

Given your design constraints (no semantic inference, deterministic pipeline ):

`section` should be a **stable URL-derived grouping key**.

Example:

```
https://docs.example.com/guide/getting-started
```

→ section = `"guide"`

```
https://docs.example.com/api/authentication/login
```

→ section = `"api"`

It is simply the first meaningful path segment (after allowed prefix).

---

### Why Section Exists

It helps with:

* Filtering in RAG (only search `api`)
* Corpus organization
* Chunk routing
* Debug grouping
* Multi-version doc separation (later extension)

It is a retrieval aid — not identity.

---

### Where It Should Be Derived

It must come from:

* Config.allowedPathPrefix
* Canonical URL path
* Deterministic path slicing

Never from:

* `<h1>`
* Breadcrumbs
* Page content
* Inferred taxonomy

---

### Deterministic Rule Example

Assume:

```
allowedPathPrefix = ["/docs"]
```

URL:

```
/docs/guide/getting-started
```

Strip prefix:

```
guide/getting-started
```

Take first segment:

```
guide
```

That becomes:

```
section: "guide"
```

---

### Edge Cases

If URL is:

```
/docs/getting-started
```

No nested grouping.

You may define:

```
section: "root"
```

Or:

```
section: ""
```

But you must define it explicitly and keep it stable.

---

### What Section Is NOT

It is not:

* Category inferred from headings
* "Introduction"
* "Advanced Topics"
* Semantic clustering label
* Version label (unless URL encodes it)

Keep it mechanical.

---

### Relationship to DocID

| Field        | Derived From       | Stable Across Content Change |
| ------------ | ------------------ | ---------------------------- |
| doc_id       | canonical URL      | Yes                          |
| section      | canonical URL path | Yes                          |
| content_hash | normalized content | No                           |

---

### Where It Belongs In Pipeline

Inside `Normalize()`:

```
canonicalURL := ...
docID := hash(canonicalURL)
section := deriveSection(canonicalURL)
contentHash := hash(normalizedBody)
```

Then inject frontmatter.

---

### Strict Definition

> Section is the deterministic, URL-path-derived logical grouping key used to organize documents within a documentation corpus.

---

## Title

In your architecture, `Title` should be derived from the **top-most heading of the normalized Markdown document**, not from arbitrary HTML metadata.

However, this must respect your sanitizer and normalization invariants.

---

### Where Title Comes From in Your Pipeline

Pipeline order:

```
Fetch
→ Extract
→ Sanitize  (guarantees single deterministic document)
→ Convert   (produces Markdown)
→ Resolve Assets
→ Normalize  ← Title derived here
```

By the time you reach `Normalize()`:

* Structure is deterministic (Sanitizer guarantees this )
* Markdown structure exists
* Heading hierarchy is stable

Therefore:

> Title = text content of the first top-level heading in Markdown.

---

### Why Not Use `<title>` Tag?

Because:

* `<title>` often includes branding (“Getting Started – MyProduct Docs”)
* It may not match document root
* It is presentation metadata, not content structure

Your system prioritizes **semantic structure over page chrome** .

So Title should come from content.

---

### Strict Rule for Title Derivation

In `Normalize()`:

1. Identify the highest-level heading in the Markdown AST
2. After heading repair (if any)
3. Extract its plain text (no formatting, no inline code markup)
4. Trim whitespace
5. Use that as `Title`

Example:

Markdown:

```
# Getting Started

## Installation
```

→

```
title: "Getting Started"
```

---

### What If There Are Multiple H1s?

Sanitizer may allow multiple H1s if structure is still provable .

But normalization stage enforces:

> Exactly one H1 per file 

If after normalization there are multiple competing roots:

→ `ErrCauseBrokenH1Invariant` (fatal)
→ mapped to `metadata.CauseInvariantViolation`

So Title extraction assumes invariant is satisfied.

---

### What If There Is No H1?

If no headings exist:

* Sanitizer should already have failed (Invariant H3) 
* Or normalization should treat this as invariant violation

You must not invent a title.

No semantic guessing.
No fallback to filename.
No fallback to URL.

Better to fail than invent wrong identity.

---

### What About Formatting Inside Heading?

Example:

```
# Installing `mytool`
```

Title should be:

```
Installing mytool
```

So:

* Strip Markdown formatting
* Preserve text content
* Avoid backticks or markup artifacts

---

### Edge Case: Nested Structure

If document starts:

```
## Getting Started
```

Sanitizer may have repaired hierarchy to ensure correct levels.

Normalization must operate on repaired structure.

Title must reflect the final normalized structure, not raw HTML.

---

### Final Formal Definition

> Title is the plain-text content of the top-most heading in the normalized Markdown document, derived after structural repair and invariant enforcement.

It is:

* Deterministic
* Content-derived
* Not semantically inferred
* Not UI-derived
* Not guessed

---
