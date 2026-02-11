**`RecordArtifact(path string)` is too weak for what the system is evolving into.** It records that *something* was written, but it doesn’t give us enough structure to support:

* multi-artifact pages (markdown + assets),
* storage failure diagnostics,
* deterministic accounting,
* incremental crawl support later,
* or post-run validation.

Below is a precise analysis grounded in your current architecture.

---

# 1️⃣ What `RecordArtifact` Currently Does

From `metadata/recorder.go`:

```go
func (r *Recorder) RecordArtifact(path string) {}
```

And the sink interface:

```go
RecordArtifact(path string)
```

From `storage/sink.go`:

```go
s.metadataSink.RecordArtifact(writeResult.artifact.path)
```

So currently:

* It logs only a single path
* It does not classify artifact type
* It does not record success vs overwrite
* It does not record content hash
* It does not relate artifact to source URL
* It does not distinguish markdown vs asset

This is underpowered relative to your design goals in the Technical Design doc  and the architecture spec .

---

# 2️⃣ Architectural Mismatch

Your design states:

> Output must be deterministic
> Asset deduplication via content hash
> Optional sidecar metadata for change detection 

But `RecordArtifact(path)` gives you no:

* hash
* overwrite info
* logical type
* linkage to page

This becomes problematic for:

### A. Asset resolution (Task 4.2)

Assets are:

* downloaded
* deduplicated by content hash
* possibly reused across pages

You need to distinguish:

* markdown artifact written
* asset written new
* asset reused (already exists)
* asset write skipped
* asset write failed

`path string` alone cannot encode this safely.

---

# 3️⃣ What You Actually Need

You need **artifact as a structured event**, not a string.

Here is the minimal shape that fits your architecture:

```go
type ArtifactKind string

const (
    ArtifactMarkdown ArtifactKind = "markdown"
    ArtifactAsset    ArtifactKind = "asset"
)

type ArtifactRecord struct {
    Kind        ArtifactKind
    WritePath   string
    SourceURL   string
    ContentHash string
    Overwrite   bool
    Bytes       int64
}
```

Then:

```go
RecordArtifact(record ArtifactRecord)
```

This gives you:

| Capability                        | Enabled? |
| --------------------------------- | -------- |
| Deterministic audit               | ✅        |
| Asset dedup metrics               | ✅        |
| Final crawl stats accuracy        | ✅        |
| Storage diagnostics               | ✅        |
| Incremental crawl support         | ✅        |
| Cross-page asset reuse visibility | ✅        |

---

# 4️⃣ Why This Does NOT Violate Your Metadata Principles

From metadata contract :

> Metadata is write-only
> Metadata must not affect control flow
> Metadata is observational only

A richer `ArtifactRecord` is still:

* Observational
* Non-influential
* Write-only

You are not leaking policy into metadata.

You're simply making metadata useful.

---

# 5️⃣ Another Important Gap: Asset vs Markdown Separation

Right now:

```go
totalAssets int
```

exists in final stats.

But where do you increment it?

You currently have no reliable structured artifact counting.

If you log artifacts as typed events:

```go
if record.Kind == ArtifactAsset { ... }
```

The scheduler can compute stats without relying on recorder state (important per your rule that stats must be derived from scheduler state, not recorder accumulation).

This preserves your invariant:

> CrawlStats MUST be derived from scheduler state, not accumulated incrementally via recorder 

---

# 6️⃣ Minimal Change Recommendation

If you want the smallest safe improvement:

### Replace:

```go
RecordArtifact(path string)
```

### With:

```go
RecordArtifact(
    kind string,
    path string,
    attrs []Attribute,
)
```

Where attrs can include:

* AttrURL
* AttrAssetURL
* AttrWritePath
* content hash (as AttrField if needed)
* size

That keeps your attribute pattern consistent with `RecordError`.

---

# 7️⃣ Even Better: Make Artifact Recording the Storage Responsibility

Right now only storage calls `RecordArtifact`.

But assets are written by asset resolver logic (even if delegated).

You should ensure:

* Only the component that performs the write emits the artifact event.
* Scheduler must not fabricate artifact metadata.

That preserves strict stage isolation described in .

---

# 8️⃣ What You Should NOT Do

Do NOT:

* Let `RecordArtifact` influence dedup logic.
* Use metadata to determine whether an asset already exists.
* Query metadata to compute crawl stats.

Metadata remains append-only observability.

---

# 9️⃣ Final Verdict

**Current design:**

```go
RecordArtifact(path string)
```

is too weak and will become a liability in Task 4.2.

It should be upgraded to a structured artifact event with:

* artifact type
* path
* source URL
* hash
* overwrite indicator
* size

Without this, your metadata layer will not support:

* reliable auditability
* asset dedup observability
* incremental crawl
* deterministic validation

---

If you want, I can now:

* Propose the exact revised `MetadataSink` interface
* Show how storage + asset resolver should call it
* Or show how scheduler should compute final stats cleanly without reading metadata

Your architecture is strong — this is just one of the few spots where observability depth must match system ambition.
