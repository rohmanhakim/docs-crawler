Below is a **complete, explicit test-fixture suite for `mdconvert` (Task 4.1)**, designed to *directly* validate the invariants you just accepted.

These fixtures are **unit-level**, not integration tests:

* **Input:** `SanitizedHTMLDoc` (already valid)
* **Output:** raw `MarkdownDoc`
* **No sanitizer, no normalize, no assets**

You should be able to run these with a **golden-file** test harness.

---

# Task 4.1 — `mdconvert` Test Fixtures

## Fixture Naming Convention

```
mdconvert_<category>_<scenario>_<expectation>
```

Each fixture consists of:

* `input.html` (sanitized HTML)
* `expected.md`
* (optional) `notes.md` explaining invariant coverage

---

## 1. Heading & Structure Fixtures

### 1.1 Single H1, Clean Hierarchy

**Fixture**

```
mdconvert_heading_single_h1_clean
```

**Covers**

* M2 (order)
* M4 (mapping)
* M7 (no validation)

**HTML**

```html
<h1>Title</h1>
<h2>Section</h2>
<p>Text</p>
```

**Expected MD**

```md
# Title

## Section

Text
```

---

### 1.2 Multiple H1 (Allowed)

**Fixture**

```
mdconvert_heading_multiple_h1_passthrough
```

**Covers**

* M7 (no heading repair)
* M10 (must not reject)

**HTML**

```html
<h1>A</h1>
<p>x</p>
<h1>B</h1>
<p>y</p>
```

**Expected MD**

```md
# A

x

# B

y
```

---

### 1.3 Skipped Heading Levels

**Fixture**

```
mdconvert_heading_skipped_levels_preserved
```

**Covers**

* M7
* M8

**HTML**

```html
<h1>Root</h1>
<h3>Deep</h3>
```

**Expected MD**

```md
# Root

### Deep
```

---

## 2. Non-Inference Fixtures (Critical)

### 2.1 Bold Is NOT a Heading

**Fixture**

```
mdconvert_no_infer_bold_heading
```

**Covers**

* M1 (non-inference)

**HTML**

```html
<p><strong>Not a heading</strong></p>
<p>Text</p>
```

**Expected MD**

```md
**Not a heading**

Text
```

---

### 2.2 Styled Paragraph Ignored Structurally

**Fixture**

```
mdconvert_no_css_semantics
```

**HTML**

```html
<p style="font-size:24px;font-weight:bold">Visual Title</p>
<p>Body</p>
```

**Expected MD**

```md
Visual Title

Body
```

---

## 3. Order & Linearization Fixtures

### 3.1 DOM Order Preserved (No Reordering)

**Fixture**

```
mdconvert_dom_order_preserved
```

**Covers**

* M2

**HTML**

```html
<p>A</p>
<ul>
  <li>1</li>
</ul>
<p>B</p>
```

**Expected MD**

```md
A

- 1

B
```

---

## 4. Code Fidelity Fixtures

### 4.1 Inline Code Preserved

**Fixture**

```
mdconvert_inline_code_verbatim
```

**Covers**

* M5

**HTML**

```html
<p>Use <code>x := 1</code></p>
```

**Expected MD**

```md
Use `x := 1`
```

---

### 4.2 Fenced Code with Language

**Fixture**

```
mdconvert_codeblock_language_preserved
```

**HTML**

```html
<pre><code class="language-go">fmt.Println("hi")</code></pre>
```

**Expected MD**

````md
```go
fmt.Println("hi")
````

```

---

### 4.3 No Language Guessing

**Fixture**
```

mdconvert_codeblock_no_language_guess

````

**HTML**
```html
<pre><code>SELECT * FROM users;</code></pre>
````

**Expected MD**

```md
```

SELECT * FROM users;

```
```

---

## 5. Table Fixtures

### 5.1 Simple Table

**Fixture**

```
mdconvert_table_basic
```

**Covers**

* M6

**HTML**

```html
<table>
<tr><th>A</th><th>B</th></tr>
<tr><td>1</td><td>2</td></tr>
</table>
```

**Expected MD**

```md
| A | B |
|---|---|
| 1 | 2 |
```

---

### 5.2 Ugly Table Still Emitted

**Fixture**

```
mdconvert_table_irregular_structure
```

**HTML**

```html
<table>
<tr><td>A</td><td>B</td><td>C</td></tr>
<tr><td>1</td></tr>
</table>
```

**Expected MD**

```md
| A | B | C |
|---|---|---|
| 1 |   |   |
```

(no inference, no dropping)

---

## 6. Links & Images (No Resolution)

### 6.1 Relative Link Preserved

**Fixture**

```
mdconvert_link_relative_passthrough
```

**HTML**

```html
<a href="../api">API</a>
```

**Expected MD**

```md
[API](../api)
```

---

### 6.2 Image Not Downloaded

**Fixture**

```
mdconvert_image_passthrough
```

**HTML**

```html
<img src="/img/logo.png" alt="Logo">
```

**Expected MD**

```md
![Logo](/img/logo.png)
```

---

## 7. Unknown / Unsupported Elements

### 7.1 Unknown Tag Drops Structure, Keeps Text

**Fixture**

```
mdconvert_unknown_tag_text_only
```

**Covers**

* M4

**HTML**

```html
<custom-box>
  <p>Hello</p>
</custom-box>
```

**Expected MD**

```md
Hello
```

---

## 8. Determinism Fixtures

### 8.1 Whitespace Stability

**Fixture**

```
mdconvert_whitespace_deterministic
```

**HTML**

```html
<p>A</p><p>B</p>
```

**Expected MD**

```md
A

B
```

Running twice MUST produce identical bytes.

---

## 9. Explicit Negative Assertions (Meta-Tests)

These are *assertions about behavior*, not fixtures:

* mdconvert MUST NOT:

  * throw on multiple H1s
  * throw on skipped levels
  * infer headings
  * reorder nodes
  * emit raw HTML
  * fail on ugly output

Any test expecting an error here is **invalid**.

---

## 10. Minimal Test Matrix Coverage

| Invariant | Covered By               |
| --------- | ------------------------ |
| M1        | no_infer_bold_heading    |
| M2        | dom_order_preserved      |
| M3        | whitespace_deterministic |
| M4        | unknown_tag_text_only    |
| M5        | codeblock_*              |
| M6        | table_*                  |
| M7        | multiple_h1_passthrough  |
| M8        | skipped_levels_preserved |
| M9        | link_*, image_*          |
| M10       | all fixtures             |

---

## Final Guidance

If **any** of these fixtures fail:

* it is a **mdconvert bug**
* not a sanitizer problem
* not a normalize problem