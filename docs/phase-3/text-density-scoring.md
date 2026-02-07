# Text Density Scoring

Use a **weighted scoring function**, not raw text length — **but keep it deliberately simple**.

This is the industry-standard compromise between correctness and determinism, and it fits Task 3.1 perfectly.

---

## Why raw text length is insufficient

A pure “longest text wins” heuristic **fails in common doc layouts**:

### Failure modes of text length alone

* Large boilerplate blocks (license text, nav dumps)
* Aggregated link sections (“See also” pages)
* Changelog / index pages with many short items
* Footer/legal sections accidentally surviving chrome removal

Text length measures **quantity**, not **semantic density**.

Industry extractors learned this the hard way.

---

## Industry-standard solution: lightweight weighted scoring

The goal is **not NLP**.
The goal is **structural signal weighting**.

### Core principle

> **Some HTML elements are stronger signals of “documentation content” than others.**

Headings, paragraphs, and code blocks are *positive evidence*.

---

## Recommended scoring model (Task 3.1–appropriate)

Use a **single-pass, additive score** per candidate node.

### Minimal signals (sufficient for docs)

| Signal                       | Why it matters     | Typical weight                 |
| ---------------------------- | ------------------ | ------------------------------ |
| Text length (non-whitespace) | Baseline size      | `+1 per 50 chars`              |
| Paragraph count (`<p>`)      | Prose density      | `+5 each`                      |
| Heading count (`<h1–h3>`)    | Structure          | `+10 each`                     |
| Code blocks (`<pre><code>`)  | Strong doc signal  | `+15 each`                     |
| List items (`<li>`)          | Medium signal      | `+2 each`                      |
| Link density penalty         | Nav/index detector | `−X if links/text > threshold` |

This is enough to beat **>90%** of real cases.

---

## What *not* to do (important)

❌ No NLP
❌ No language detection
❌ No TF-IDF
❌ No DOM depth tricks
❌ No CSS-based layout inference

Those reduce determinism and are overkill for Task 3.1.

---

## How industry systems actually apply this

The algorithm is conceptually:

```
for each candidate node after chrome removal:
    score = 0
    score += textLengthScore(node)
    score += paragraphScore(node)
    score += headingScore(node)
    score += codeBlockScore(node)
    score -= linkDensityPenalty(node)

choose node with highest score
```

With one **hard gate**:

> If the best score is below a minimal threshold → **no content found**

---

## Why this fits the architecture perfectly

* **Extractor**:

  * Chooses the content root using this scoring
* **Sanitizer**:

  * Cleans the chosen subtree
* **Normalize**:

  * Enforces invariants later

No stage overlap.

---

## Concrete guidance for Task 3.1 (decision)

### Do this

* Implement a **weighted structural scoring function**
* Keep weights static and deterministic
* Prefer simplicity over cleverness

### Do not do this

* Do not rely on raw text length alone
* Do not introduce probabilistic or ML logic
* Do not prematurely optimize weights

---

## One-line final answer

> **Use a simple weighted scoring function that favors headings, paragraphs, and code blocks over raw text length; pure text length alone is insufficient and non-robust.**