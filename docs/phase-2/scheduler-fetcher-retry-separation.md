# Separation between Scheduler-level retry with Fetcher's internal retry

**The fetcher must NOT know about, read from, or coordinate with the limiter’s state.**

This separation is **intentional and required** in the design.

---

## Clean separation of concerns (this is the key)

### Scheduler + Limiter own **when** a fetch is allowed to start

They handle:

* robots.txt `crawl-delay`
* host-level politeness
* backoff after robots.txt failures
* per-host pacing across the *entire crawl*

This answers:

> *“When may we send the next request to this host?”*

---

### Fetcher owns **how** a single fetch attempt is executed

It handles:

* HTTP retries within one logical fetch
* retry count
* per-attempt backoff (local, bounded)
* retryable vs non-retryable transport failures

This answers:

> *“Given permission to fetch now, can I complete this request successfully?”*

These are orthogonal problems.

---

## Why the fetcher must not see limiter data

If the fetcher knew about limiter state, it would introduce **cross-layer coupling** and subtle bugs.

### 1. It would double-apply delays

* Scheduler delays before calling `Fetch`
* Fetcher delays again based on limiter data

This results in:

* unpredictable latency
* broken politeness guarantees
* nondeterministic timing

---

### 2. Ownership of “when” would leak

The architecture is very clear:

> **The scheduler is the sole control-plane authority.**

If the fetcher reads limiter data, it implicitly starts making *scheduling* decisions.

That violates:

* determinism
* testability
* single-responsibility boundaries

---

### 3. It breaks future concurrency safety

In a multi-worker future:

* limiter state is shared
* fetchers are per-worker

Letting fetchers read limiter state would require:

* synchronization
* locking
* cross-worker coordination

All of that belongs in the scheduler / limiter layer, not the fetcher.

---

## Correct interaction model (this is what the system want)

```text
Scheduler
  └── Limiter.Wait(host)        ← global, cross-request pacing
  └── Fetcher.Fetch(url)
        ├── attempt #1
        ├── local retry backoff (internal, bounded)
        ├── attempt #2
        ├── attempt #3
        └── return result
```

Key points:

* Limiter delay happens **once**, before fetch starts
* Fetcher retries happen **inside one logical fetch**
* No shared state between limiter and fetcher

---

## What kind of backoff is allowed inside the fetcher?

Fetcher backoff must be:

* **local**
* **short-lived**
* **bounded**
* **self-contained**

Examples:

* exponential backoff capped at a few seconds
* honoring `Retry-After` header *for this request only*

It must **not**:

* update host-level delay
* influence future requests
* persist state across fetch calls

---

## How the two retry layers complement each other

| Layer               | Scope                   | Purpose                  |
| ------------------- | ----------------------- | ------------------------ |
| Limiter (scheduler) | Cross-request, per-host | Politeness & crawl delay |
| Fetcher retry       | Single request          | Transport robustness     |

They solve different problems and must not merge.

---

## One-sentence rule to rely on

> **The fetcher retries *within* a granted fetch window; the scheduler+limiter decide *when* that window exists.**

If the system keep that rule, it stays:

* deterministic
* polite
* composable
* concurrency-safe

---

## Final verdict

> ✅ The scheduler correctly handles crawl-delay and host backoff via the limiter.
> ❌ The fetcher must NOT know about the limiter’s data.
> ✅ Fetcher retries are local, bounded, and ignorant of global pacing.