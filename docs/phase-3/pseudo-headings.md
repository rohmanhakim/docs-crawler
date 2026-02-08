# Pseudo-Heading

**Pseudo-heading should be out of scope for Task 3.2**.

Longer, precise rationale below.

---

## Why pseudo-heading heuristics are **out of scope** for S2

Under **Invariant S2 — No Semantic Inference**, the sanitizer is explicitly forbidden from *guessing author intent* or *inferring structure from presentation* . Detecting pseudo-headings like:

* `<strong>` blocks acting as section titles
* ALL-CAPS paragraphs implying headings
* Font-size / font-weight–based structure
* Styled `<p>` elements used instead of `<h*>`

**necessarily relies on semantic or visual inference**, not provable DOM structure.

That immediately violates S2.

### Why heuristics are disallowed here

A heuristic such as:

* “bold-only paragraph → heading”
* “short ALL-CAPS line → section title”

requires at least one of:

* Interpreting **visual intent**
* Interpreting **author conventions**
* Choosing among multiple plausible interpretations

All of these are explicitly forbidden by the sanitizer contract:

> *“If a repair requires guessing author intent → fail.”* 

Once we allow these heuristics, the sanitizer is no longer a **structural gatekeeper**; it becomes a **semantic rewriter**, which the design explicitly rejects.

---

## Correct behavior in Task 3.2

If a page has **no real headings** (`h1–h6`) and relies on pseudo-headings:

* If **no equivalent structural anchors can be proven** → **FAIL**
* Return `ErrCauseStructurallyIncorrect`
* Do **not** attempt best-effort reconstruction

This is explicitly mandated by **Invariant H3**:

> *“If no headings exist and no equivalent structural anchors can be proven, sanitizer MUST fail.”* 

This is not a limitation — it is a **deliberate correctness boundary**.

---

## Where this *could* belong (but not now)

If we ever choose to support pseudo-headings, it must be:

* **Outside the sanitizer**
* In an **optional, explicitly heuristic stage**
* Clearly marked as **non-deterministic / best-effort**
* Opt-in, not default

Candidates (future work, not Task 3.2):

* An LLM-assisted post-processor
* A separate “semantic enrichment” phase
* A documentation-specific adapter layer

But **not** in the sanitizer.

---

## Design principle reaffirmed

From the invariant document:

> **“It is always better to reject an ambiguous document than to invent a wrong one.”** 

Detecting pseudo-headings *invents structure*.
Task 3.2 exists specifically to prevent that.

---

### Final recommendation

✔ **Out of scope for Task 3.2**
✔ **Do not add heuristics for `<strong>` / styled paragraphs**
✔ **Fail fast with `ErrCauseStructurallyIncorrect` when structure is unprovable**

This keeps the sanitizer **correct, deterministic, and defensible**, which is exactly its role in the pipeline.
