# Structural Rules

**normalize** is structurally *after*:

* extractor → isolates content
* sanitizer → guarantees structural determinism
* mdconvert → maps HTML → Markdown faithfully
* assets → rewrites references

So the structural rules enforced here are **not about DOM validity anymore**.

They are about **document-level invariants required for RAG stability**.

Let’s be precise.

---

## First: What Was Already Guaranteed Before Normalize?

From your sanitizer invariants :

Sanitizer guarantees:

* Exactly one deterministic document
* Linearizable reading order
* Single document root
* No ambiguous heading structure
* No semantic invention
* Repair is structural only

So by the time you enter `normalize`, you already have:

✔ A structurally valid document
✔ Stable reading order
✔ Headings in deterministic order
✔ No ambiguous roots

Therefore:

> Normalize MUST NOT re-litigate structural ambiguity.
> That was Task 3.2’s responsibility.

---

## Then What Does Normalize Enforce?

Now refer to your design doc for Markdown Normalization  and .

Normalize responsibilities:

* Inject frontmatter
* Enforce structural rules
* Prepare for RAG chunking

These structural rules are **not HTML-structure rules**.

They are **document contract rules for storage + retrieval**.

---

## Structural Rules Enforced in Normalize (Task 5.1)

These differ from sanitizer rules in a key way:

| Sanitizer               | Normalize                 |
| ----------------------- | ------------------------- |
| Valid DOM               | Valid Markdown document   |
| Deterministic structure | RAG-stable structure      |
| No semantic guessing    | Enforce ingest invariants |
| Single root             | Exactly one canonical H1  |

Now let’s define them clearly.

---

## Structural Rule N1 — Exactly One Canonical H1

Even though sanitizer guarantees single document root, it does NOT enforce “exactly one H1”.

Your design explicitly says:

> One H1 per document 

Normalize must:

* Ensure there is exactly one top-level H1
* If zero → error
* If more than one → error (`ErrCauseBrokenH1Invariant`)
* This maps to `metadata.CauseInvariantViolation`

Why here and not sanitizer?

Because:

* Sanitizer checks *structural provability*
* Normalize checks *Markdown contract correctness*

These are different layers of invariants.

---

## Structural Rule N2 — No Skipped Heading Levels

Sanitizer MAY renumber headings for structural correctness.

Normalize enforces:

* No level skipping (H1 → H3 is invalid)
* Hierarchy must increment by at most +1
* Downgrades are allowed (H3 → H2)

This guarantees stable section trees for chunking.

If violated → normalization error.

---

## Structural Rule N3 — No Empty Sections

Markdown sections like:

```
### Something
```

with no content beneath them before next heading → invalid.

Why?

Because chunkers depend on:

```
Heading + content block
```

Empty sections cause:

* Degenerate chunks
* Anchor-only documents
* Retrieval noise

Sanitizer does not enforce this.
Normalize must.

---

## Structural Rule N4 — No Content Outside Root Hierarchy

After conversion, you must ensure:

* No stray paragraphs before first H1
* No content after structure terminates
* No orphaned blocks

Everything must belong to the single document tree.

Sanitizer ensures structure.
Normalize ensures **Markdown-level tree integrity**.

---

## Structural Rule N5 — Atomic Blocks Are Preserved

From the design :

> Code blocks and tables are atomic

Normalize must ensure:

* No splitting of fenced blocks
* No heading insertion inside code
* No chunk boundaries within tables

Sanitizer doesn’t deal with Markdown fences.
Normalize must validate this.

---

## Structural Rule N6 — Stable Top-Level Title Binding

Frontmatter `title:` must match canonical H1.

Normalize must:

* Extract canonical H1 text
* Bind it to frontmatter title
* Ensure mismatch cannot occur

Title is not “metadata guess”.
It is derived from structural truth.

---

## What Normalize Must NOT Do

It must NOT:

* Infer missing H1
* Create headings
* Merge sections
* Repair semantic issues
* Reorder content

Those would violate sanitizer invariants .

Normalize only enforces contract-level invariants.

---

## Conceptual Difference (Very Important)

Sanitizer answers:

> “Can this HTML be proven to represent exactly one document?”

Normalize answers:

> “Is this Markdown document structurally valid for storage and RAG ingestion?”

They operate at different abstraction layers.

---

## Where Errors Map

Your normalize error currently:

```go
ErrCauseBrokenH1Invariant
```

maps to:

```go
metadata.CauseInvariantViolation
```

Correct. That is exactly appropriate.

Because this is not content invalidity.
It is a system-level contract violation.

---

## Final Summary — Structural Rules in Task 5.1

After frontmatter extraction, normalize must enforce:

1. Exactly one canonical H1
2. No skipped heading levels
3. No empty sections
4. No orphaned content outside hierarchy
5. Atomic preservation of code blocks and tables
6. Frontmatter title matches H1

These differ from sanitizer because:

* Sanitizer enforces DOM structural provability.
* Normalize enforces Markdown contract integrity for RAG stability.

---

## Mutations

Short answer: **yes — effectively no semantic mutations at all**, with one important clarification.

Normalize is a **constraint gate**, not a transformation engine.

After sanitizer (Task 3.2), structure is already provable .
After mdconvert, Markdown reflects that structure faithfully .

So normalize must not “improve” content.

---

### What Normalize Is Allowed To Mutate

Only two categories:

#### 1️⃣ Frontmatter Injection (Required)

This is the primary mutation:

* Inject YAML block
* Bind `title` to canonical H1
* Inject `source_url`
* Inject `depth`
* Inject `section`

This is structural metadata, not content mutation.

---

#### 2️⃣ Deterministic Structural Normalization (Very Limited)

These are allowed only if they:

* Do not reorder nodes
* Do not invent structure
* Do not change semantics

Examples:

##### ✔ Allowed (Purely Mechanical)

* Trim trailing whitespace
* Normalize multiple blank lines to one
* Ensure newline before headings
* Ensure file ends with newline
* Normalize heading syntax (e.g., `#Title` → `# Title`)
* Remove empty sections (if and only if they are provably empty and not meaningful)

These are formatting stabilizations — not semantic mutations.

---

### What Normalize Must NOT Mutate

Anything that changes meaning or hierarchy:

🚫 Renaming headings
🚫 Inserting missing H1
🚫 Merging sections
🚫 Reordering blocks
🚫 Rewriting links
🚫 Splitting long sections
🚫 Fixing “bad writing”
🚫 Inserting chunk markers

Those would violate:

* Sanitizer invariants 
* Conversion fidelity principles 

---

### Important Distinction

There are two kinds of mutation:

#### 1. Structural repair mutation

Allowed only in sanitizer.

#### 2. Contract enforcement mutation

Allowed in normalize only if mechanical and deterministic.

Normalize is a validator with minimal canonicalization.

---

### The Clean Mental Model

Think of normalize as:

```
assert(valid_markdown_contract)
+ inject(frontmatter)
+ canonicalize_formatting()
```

If any invariant fails:

→ Return error
→ Do not attempt repair

It is better to reject than mutate incorrectly.

---

### So Precisely

No mutations besides:

* Frontmatter injection
* Deterministic formatting canonicalization

No semantic changes.
No hierarchy changes.
No content changes.

If you feel the need to “fix” content, that belongs in sanitizer — not normalize.

---

