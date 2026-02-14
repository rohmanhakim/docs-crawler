# WriteResult

`WriteResult` defines your **storage boundary contract**.

> `WriteResult` should not be “just metadata.”
> It should represent the **persistence outcome of a single document write operation**.

Right now you have:

```go
type WriteResult struct {
    artifact Artifact
}

type Artifact struct {
    path string
}
```

That is under-specified for what your architecture actually needs.

Let’s design it properly.

---

# 1. What WriteResult Represents (Architecturally)

From your storage contract :

Storage is responsible for:

* Persist Markdown files
* Write assets
* Ensure deterministic filenames
* Idempotent writes
* Overwrite-safe reruns

So `WriteResult` must answer:

> “What exactly was persisted as a result of this document?”

It is not logging metadata.
It is not crawl statistics.
It is not an observability record.

It is the **artifact-level write outcome**.

---

# 2. What WriteResult Should Contain

At minimum:

## A. Artifact Identity

```go
type WriteResult struct {
    URLHash string        // identity (filename without extension)
    Path    string        // absolute or relative path written
}
```

This lets scheduler:

* Aggregate results
* Surface outputs to caller
* Potentially compare expected vs actual

---

## B. Content Hash (Strongly Recommended)

```go
ContentHash string
```

This allows:

* Change detection
* Incremental recrawl later
* Testing determinism

Important: this is NOT used for filename.

---

## C. Asset Count (Optional but Useful)

Since asset resolver may download images:

```go
AssetCount int
```

This helps scheduler compute final stats:

```go
totalAssets
```

Which your finalizer expects.

---

## D. Overwrite Status (Optional)

If you later support incremental mode:

```go
WasOverwritten bool
WasCreated     bool
```

This allows smart reporting.

---

# 3. What It Should NOT Contain

It must NOT contain:

* Crawl depth
* Source URL (belongs in frontmatter)
* ErrorCause
* Retry logic
* Metadata structures
* Timestamps (metadata handles that)

Keep it strictly persistence-related.

---

# 4. Separation of Concerns

Important boundary:

| Concept          | Lives In                |
| ---------------- | ----------------------- |
| Observability    | metadata                |
| Control flow     | scheduler               |
| Identity         | storage                 |
| Determinism      | canonical URL + storage |
| Chunking         | normalize               |
| HTML correctness | sanitizer               |

`WriteResult` belongs strictly to **storage**.

---

# 5. Recommended Structure

Clean, future-proof version:

```go
type WriteResult struct {
    URLHash     string
    Path        string
    ContentHash string
    AssetCount  int
}
```

You may also wrap artifact:

```go
type Artifact struct {
    URLHash     string
    Path        string
    ContentHash string
}
```

But avoid nesting too much unless you expect multiple artifacts per document.

---

# 6. Why It Should Not Be “Just Metadata”

Metadata is:

* Write-only
* Observational
* Not used by control flow 

`WriteResult` is returned to scheduler and stored in:

```go
s.writeResults = append(s.writeResults, writeResult)
```

This is part of crawl execution output:

```go
type CrawlingExecution struct {
    WriteResults []storage.WriteResult
}
```

That means it is part of the **execution result contract**, not logging.

If it were “just metadata,” you would not need to return it.

---

# 7. Minimal vs Correct

If you want absolute minimal version:

```go
type WriteResult struct {
    Path string
}
```

That technically works.

But architecturally, the correct design for your system — especially given future incremental crawl and change detection — is to include:

* URLHash
* ContentHash
* Path
* AssetCount

---

# 8. Final Answer

`WriteResult` should represent:

> The concrete, deterministic, persisted artifact identity and outcome of writing a normalized document to storage.

It is NOT:

* A log entry
* A metadata record
* A scheduler control object

It is a persistence-layer result object.

---