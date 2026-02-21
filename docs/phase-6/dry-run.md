# Dry Run

Good. Let’s define a **precise DryRun contract** that is architecturally clean and doesn’t rot your control-plane design.

We’ll base this strictly on your existing boundaries and invariants  .

---

# Dry Run — Formal Contract

## Definition

> DryRun executes the full crawl control-plane and pipeline, using real network fetch and real DOM processing, but suppresses irreversible storage side effects.

It does **not** alter scheduling logic.
It does **not** bypass pipeline stages.
It does **not** change retry/abort semantics.

---

# What Dry Run MUST Do

### 1. Full Scheduler Loop

Everything inside:

```go
for {
    nextPolicy, ok := s.frontier.Dequeue()
    ...
}
```

runs normally.

This includes:

* robots admission
* frontier traversal
* fetch
* extract
* sanitize
* markdown convert
* normalize
* scheduler error handling
* stats aggregation

Control-plane remains untouched.

---

### 2. Real Fetching

Keep:

```go
s.htmlFetcher.Fetch(...)
```

Why?

* You need real link discovery
* You need real DOM validation
* You need sanitizer invariants enforced 
* You need real depth and scope validation

Network is not an irreversible mutation.

---

### 3. Suppress Storage Writes

This is the only component that must change behavior.

Instead of writing files:

```go
storageSink.Write(...)
```

Dry run should:

* Compute filename
* Record metadata
* Return a WriteResult
* NOT touch filesystem

---

# Correct Implementation Strategy

Do NOT do:

```go
if cfg.DryRun {
    return
}
```

inside Scheduler.

That couples control-plane to mode branching.

Instead:

Inject different Storage implementation.

---

## Step 1 — Define Storage Interface (already exists)

You already have:

```go
type Sink struct { ... }
func (s *Sink) Write(...) (WriteResult, internal.ClassifiedError)
```

---

## Step 2 — Introduce DryRunSink

```go
type DryRunSink struct {
    metadataSink metadata.MetadataSink
}

func (d *DryRunSink) Write(
    normalizedDoc normalize.NormalizedMarkdownDoc,
) (WriteResult, internal.ClassifiedError) {

    // Derive deterministic path exactly as normal sink would
    path := derivePath(normalizedDoc)

    // Emit artifact metadata
    d.metadataSink.RecordArtifact(path)

    // Return WriteResult
    return WriteResult{
        artifact: Artifact{path: path},
    }, nil
}
```

Important:

* Same filename derivation logic
* Same metadata emission
* No disk write

Scheduler remains unchanged.

---

## Step 3 — Inject Based on Config

In `NewScheduler`:

```go
if cfg.DryRun() {
    storageSink = storage.NewDryRunSink(&recorder)
} else {
    storageSink = storage.NewSink(&recorder)
}
```

That is the only branching point.

---

# What Dry Run Output Should Print

At end of crawl:

You already have:

```go
RecordFinalCrawlStats(...)
```

Dry run should print:

```
DRY RUN SUMMARY

Pages processed: X
Errors: Y
Assets resolved: Z
Duration: T

No files were written.
```

Stats must be derived from scheduler state, not metadata accumulation .

---

# What Dry Run MUST NOT Do

* Must NOT skip frontier traversal
* Must NOT skip sanitizer
* Must NOT skip normalize
* Must NOT skip robots
* Must NOT alter retry logic
* Must NOT change error severity handling
* Must NOT depend on metadata for behavior

---

# Why This Is Architecturally Correct

Because:

* Scheduler remains sole control-plane authority 
* Metadata remains observational only 
* Determinism guarantees are preserved
* Invariants are still enforced 
* Only irreversible side effect (disk write) is suppressed

---

# Optional Enhancement (High Value)

You may also want:

* Disable asset downloads in DryRun
* But still count assets
* Still rewrite links

Same pattern:
Inject `DryRunAssetResolver` that:

* Records asset URLs
* Does not download
* Returns deterministic filenames

---

# Final Architecture Summary

DryRun is not a special crawl.
It is the same crawl with:

* Real control-plane
* Real discovery
* Real validation
* No persistent output

That keeps your system clean.
