# HTTP Retry Types

The fetcher should **retry internally only for *transport-level, transient failures*** — cases where **another immediate HTTP attempt has a reasonable chance of succeeding without changing crawl state**.

Everything else must be surfaced to the scheduler as a *final outcome*.

---

## The golden rule

> **Fetcher retries when the problem is “the network or server flaked”.
> Scheduler decides when the problem is “this URL should be revisited later (or not at all)”.**

---

## Responses the fetcher SHOULD retry internally

These are **pure HTTP / transport concerns**.

### 1. Network / I/O failures (no HTTP response)

Retry **inside the fetcher**:

* DNS resolution failure
* TCP connection reset
* TLS handshake failure
* Request timeout
* Connection closed mid-response

**Why:**
No semantic information exists yet. This is classic retry territory.

---

### 2. HTTP `5xx` (server errors)

Retry internally:

* `500 Internal Server Error`
* `502 Bad Gateway`
* `503 Service Unavailable`
* `504 Gateway Timeout`

**Why:**
These explicitly signal *temporary server failure*. Retrying immediately with backoff is standard practice.

---

### 3. HTTP `429 Too Many Requests` **(conditionally)**

Retry internally **only if**:

* `Retry-After` header is present **or**
* the fetcher has a bounded exponential backoff policy

**Why:**
`429` is a rate-limit signal, not a semantic failure. The fetcher understands backoff and timing; the scheduler must not.

> Important:
> If retry attempts are exhausted, return a **recoverable FetchError** to the scheduler — do *not* keep retrying forever.

---

### 4. Transient redirect failures

Retry internally when:

* redirect target times out
* redirect chain fails due to network error

But **not** when:

* redirect limit is exceeded (that’s terminal)

---

## Responses the fetcher MUST NOT retry internally

These must be passed to the scheduler as final outcomes.

### 1. HTTP `4xx` (client / policy errors)

Do **not** retry internally:

* `400 Bad Request`
* `401 Unauthorized`
* `403 Forbidden`
* `404 Not Found`
* `410 Gone`

**Why:**
These indicate *semantic or policy failure*. Retrying immediately is pointless.

Scheduler decides:

* skip
* abort domain
* continue crawl

---

### 2. HTTP `304 Not Modified`

Never retry internally.

**Why:**
No body, no cache reuse in current design → terminal, non-retryable.

---

### 3. Content validation failures

Do **not** retry:

* Non-HTML `Content-Type`
* Empty body
* Undecodable response

**Why:**
The server responded correctly — the content is simply unusable.

---

### 4. Redirect limit exceeded

Do **not** retry.

**Why:**
That’s a deterministic configuration failure, not a transient condition.

---

## How this maps cleanly to the error model

Inside the fetcher:

* **Retryable internally**

  * network errors
  * `5xx`
  * `429` (bounded)

* **Returned to scheduler**

  * after retries exhausted → `FetchError{Retryable: true}`
  * immediately for non-retryable cases → `FetchError{Retryable: false}`

The scheduler then decides:

* re-enqueue later
* skip
* abort crawl

---

## One-screen decision table

| Condition             | Retry in fetcher? | Why                   |
| --------------------- | ----------------- | --------------------- |
| Network timeout / DNS | ✅                 | Transient             |
| HTTP 5xx              | ✅                 | Server instability    |
| HTTP 429              | ✅ (bounded)       | Rate limiting         |
| HTTP 403 / 404        | ❌                 | Policy / semantic     |
| HTTP 304              | ❌                 | No body               |
| Non-HTML content      | ❌                 | Content invalid       |
| Redirect loop         | ❌                 | Deterministic failure |

---

## Final one-sentence rule

> **If another immediate HTTP attempt could succeed without changing crawl state, retry in the fetcher; otherwise, return the result to the scheduler.**

That rule is exactly why the `retryCount` belongs in `RecordFetch` and nowhere else.

