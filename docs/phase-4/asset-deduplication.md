# Asset Deduplication

For **Task 4.2 (assets)**, you should **not** do *semantic* deduplication before asset fetching, but you **should** do **mechanical, identity-based deduplication** *before* issuing network requests.

Below is the precise rule set that fits your architecture and invariants.

---

## The Correct Deduplication Model for Assets

### 1. What you MAY deduplicate *before* fetching assets

You **should** deduplicate **asset URLs by canonical identity** *before download*:

* Exact URL match after:

  * Resolving relative → absolute
  * Normalizing scheme/host/path
* Same page references the same image multiple times
* Multiple pages reference the same absolute asset URL

This is **purely mechanical deduplication** and is correct.

**Why this is allowed**

* No semantic assumptions
* No content inference
* No dependency on fetch results
* Deterministic and repeatable

This aligns with the Asset Manager’s responsibility:

> “Resolve asset URLs → download assets → deduplicate via content hashing” 

You are simply avoiding redundant fetches.

---

### 2. What you MUST NOT deduplicate before fetching

You **must not** deduplicate assets **by content** before fetching:

* No hashing
* No byte comparison
* No filename-based guesses
* No heuristics like “same basename”

This is because:

* Content identity is unknowable pre-fetch
* Different URLs may serve different bytes
* CDNs, cache-busters, and query params exist
* Dedup-by-content is explicitly an **asset-stage concern**

The design explicitly places **content-hash deduplication after download** .

---

### 3. Where this logic belongs (important)

**Do NOT move this upstream.**

* ❌ Not in sanitizer
* ❌ Not in mdconvert
* ❌ Not in scheduler or frontier

It belongs **inside `assets.Resolver`**, because:

* Scheduler deduplicates *documents* (URLs) 
* Frontier deduplicates *crawl targets* only 
* Asset identity is **artifact-level**, not crawl-level

---

## Recommended Two-Phase Asset Dedup Strategy

### Phase A — Pre-fetch (cheap, mandatory)

Deduplicate by:

* Canonical absolute URL
* In-memory set or map during the crawl run

Result:

* Avoid redundant HTTP requests
* Preserve determinism

### Phase B — Post-fetch (authoritative)

Deduplicate by:

* Content hash (e.g. SHA-256)
* Stable filename derived from hash

Result:

* Multiple URLs → single stored asset
* Markdown references rewritten accordingly

This exactly matches the stated asset policies .

---

## One-Line Rule to Remember

> **You may deduplicate by identity before fetching, but only deduplicate by content after fetching.**

Anything else violates your pipeline’s responsibility boundaries.
