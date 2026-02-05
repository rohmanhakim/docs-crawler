For **task 2.3**, the correct and future-proof choice is:

## **Use `http.NewRequestWithContext()` — always**

This is not an implementation detail; it is a **control-plane boundary decision** that directly affects cancellation, timeouts, retries, and determinism.

---

## Why `http.NewRequestWithContext()` is mandatory

The crawler design explicitly requires:

* bounded fetch timeouts
* retry caps and backoff
* graceful shutdown
* future concurrency (worker pools)
* scheduler-owned control flow

All of those require **context propagation**.

Using `http.NewRequest()` would force the fetcher to:

* invent its own cancellation logic ❌
* rely on `http.Client.Timeout` only ❌
* be unable to respond to scheduler aborts ❌

That violates the design principle that **pipeline stages must not control crawl lifecycle** .

---

## Who should provide `context.Context`?

### **The Scheduler provides it. Always.**

**Rule of thumb**:

> Whoever decides *whether work should continue* owns the context.

In the architecture:

* The **scheduler** is the sole control-plane authority
* The **fetcher** is a pure execution stage

This is explicitly stated in the scheduler contract:

> “Pipeline stages may detect and classify failure, but must never decide retry, continuation, or abortion.” 

So:

| Component | Responsibility                        |
| --------- | ------------------------------------- |
| Scheduler | Create, cancel, deadline, retry scope |
| Fetcher   | Consume context, never derive policy  |

---

## Correct API shape (recommended)

### Fetcher interface

```go
func (h *HtmlFetcher) Fetch(
    ctx context.Context,
    url url.URL,
) (FetchResult, internal.ClassifiedError)
```

### Scheduler usage

```go
ctx, cancel := context.WithTimeout(parentCtx, cfg.FetchTimeout())
defer cancel()

result, err := s.htmlFetcher.Fetch(ctx, nextPolicy.GetURL())
```

### Inside fetcher

```go
req, err := http.NewRequestWithContext(
    ctx,
    http.MethodGet,
    url.String(),
    nil,
)
```

This keeps **all policy outside the fetcher**.

---

## What goes into the context?

The scheduler may layer contexts to encode *mechanical* control only:

* request timeout
* retry attempt lifetime
* crawl abort signal
* worker shutdown

**Never** put semantic signals (robots, scope, depth) into context — those belong to scheduler logic, not transport.

---

## How this aligns with the design documents

* Fetcher is a **pure HTTP boundary** 
* Scheduler is the **only retry / abort authority** 
* Determinism requires controlled cancellation and bounded execution 

Using `context.Context` passed from the scheduler is the only design that satisfies all three simultaneously.

---

## Final answer (concise)

* ✅ Use **`http.NewRequestWithContext()`**
* ✅ **Scheduler creates and owns `context.Context`**
* ❌ Fetcher must not invent contexts
* ❌ Fetcher must not decide timeouts or cancellation
