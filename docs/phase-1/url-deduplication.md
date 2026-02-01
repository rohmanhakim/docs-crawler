# URL Deduplication

### What it is

**Deduplication is a stateful membership check** over canonical URLs.

```
Canonical URL ──► Seen before? yes / no
```

### Example

```
normalize("https://docs.example.com/guide/")
→ https://docs.example.com/guide

normalize("https://docs.example.com/guide#index")
→ https://docs.example.com/guide
```

Deduplication then says:

- First submission → enqueue
- Second submission → drop

### Properties

Deduplication is:

- **Stateful**
- **Order-sensitive**
- **Time-dependent**
- **Purely mechanical**

### Ownership in this system

**Owner:** `frontier`

Why:

- Frontier owns crawl ordering and history
- Deduplication is a *queue identity concern*
- It must not depend on policy or semantics

This matches the invariant:

> “Frontier Responsibilities: Maintain BFS ordering, Deduplicate URLs.”

------

## 6. One-sentence mental model (use this)

> **Deduplication enforces uniqueness of that identity over time.**