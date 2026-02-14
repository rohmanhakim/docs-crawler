# HTTP Client

**a single shared `*http.Client` is the correct design** for your crawler.

Given your architecture and determinism goals, it is not just “fine,” it is the *idiomatic and operationally correct* choice in Go.

Now let’s evaluate this rigorously in the context of your pipeline.

---

# 1. Go Semantics: `http.Client` Is Meant to Be Reused

From Go’s net/http design:

* `http.Client` is **safe for concurrent use**
* It maintains:

  * TCP connection pooling
  * Keep-alive reuse
  * Idle connection reuse
  * HTTP/2 multiplexing (when enabled)

Creating a new `http.Client` per request:

* Defeats connection pooling
* Increases TLS handshakes
* Increases latency
* Increases risk of socket exhaustion
* Violates performance best practice

So purely from Go engineering discipline:

> Reuse a single `*http.Client`.

---

# 2. Fit With Your Crawler Architecture

From your technical design document :

> “The fetcher never parses content; it only returns bytes and metadata.”

This means:

* The fetcher is a boundary adapter
* It should depend on an injected HTTP transport
* It should not own networking lifecycle

That aligns perfectly with:

* One client injected from the scheduler
* Shared across fetcher and asset resolver

---

# 3. Where Should the Client Live?

Correct layering in your system:

```
Scheduler (control plane)
    └── constructs http.Client
        └── injects into:
            - fetcher.HtmlFetcher
            - assets.Resolver
            - robots.Robot (if it fetches robots.txt)
```

The `http.Client` should:

* Be constructed once in `Scheduler`
* Configured using `config.Config`
* Passed downward
* Never created inside `fetcher` or `assets`

Creating it inside `fetcher` would violate your strict package boundary rules.

---

# 4. Why This Is Especially Important For You

Your crawler design emphasizes:

From the design document :

* Deterministic runs
* Politeness
* Low concurrency
* Bounded backoff
* Per-host behavior

A shared client enables:

### 1. Connection reuse (important even at low concurrency)

Even at concurrency 1–3, keep-alive matters.

### 2. Centralized transport tuning

You may later configure:

```go
Transport{
    MaxIdleConnsPerHost: 2,
    IdleConnTimeout: 30 * time.Second,
    TLSHandshakeTimeout: 5 * time.Second,
}
```

This belongs in one place.

### 3. Observability hook potential

If later you want:

* custom RoundTripper
* metrics instrumentation
* trace IDs

That requires a shared transport.

---

# 5. When Would Multiple Clients Be Justified?

Only if:

* You intentionally separate network isolation domains
* You need different TLS configs
* You need different proxy behavior
* You need different timeout strategies
* You need to prevent asset downloads from affecting page fetch pooling

For your current architecture:

None of these apply.

So one client is correct.

---

# 6. Subtle Design Question: Where Should Backoff Live?

Important nuance:

`http.Client` **must not** implement your backoff logic.

Backoff belongs in:

* Scheduler
* Or a retry wrapper

Your design already states:

> Backoff state is tracked per host, not globally 

So:

* Client = transport
* Scheduler = control policy
* Fetcher = boundary adapter

That separation is clean.

---

# 7. Recommended Construction Pattern

In `Scheduler`:

```go
transport := &http.Transport{
    MaxIdleConns:        10,
    MaxIdleConnsPerHost: 3,
    IdleConnTimeout:     30 * time.Second,
}

client := &http.Client{
    Timeout:   cfg.Timeout(),
    Transport: transport,
}

fetcher := fetcher.NewHtmlFetcher(client, metadataSink)
resolver := assets.NewResolver(client, metadataSink)
robot := robots.NewRobot(client, metadataSink)
```

Note:

* Inject `*http.Client`
* Do not let lower packages create it

---

# 8. One More Important Thing

Do **not**:

```go
client := http.Client{}
```

Always use a pointer:

```go
client := &http.Client{}
```

Reason:

* Avoid copying internal mutex state
* Avoid accidental transport duplication
* Ensure shared connection pool

---

# 9. Final Verdict

For your crawler:

✔ Yes — one shared `*http.Client`
✔ Constructed once in scheduler
✔ Injected downward
✔ Used by fetcher, robots, assets
✔ Backoff handled outside it

This aligns with:

* Go idioms
* Your determinism guarantees
* Your strict control-plane design
* Your package boundary discipline
* Your observability constraints

---

If you'd like, I can next analyze:

* Whether your asset resolver should reuse the same client
* Or whether robots should share the same transport
* Or how to design a retry wrapper without violating your `ClassifiedError` model
