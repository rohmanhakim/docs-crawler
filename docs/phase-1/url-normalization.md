## 1. URL normalization

### What it is

**Normalization is a deterministic transformation** that maps *many equivalent URL spellings* to **one canonical form**.

```
Raw URL(s) ──► Canonical URL
```

### Examples

All of these:

```
https://docs.example.com/guide/
https://docs.example.com/guide
https://docs.example.com/guide#index
https://docs.example.com/guide?utm_source=twitter
```

may normalize to:

```
https://docs.example.com/guide
```

### Properties

Normalization must be:

- **Pure** (no state, no memory)
- **Deterministic**
- **Idempotent**
  `normalize(normalize(url)) == normalize(url)`
- **Context-free** (does not depend on crawl history)

### Ownership in your system

**Owner:** `scheduler` (Task 1.1)

Why:

- Normalization defines **URL identity**
- Identity is a *semantic admission concern*
- Frontier is forbidden from redefining identity

This aligns with:

> “Scheduler is the ONLY component allowed to decide whether a URL may enter the crawl frontier.”

