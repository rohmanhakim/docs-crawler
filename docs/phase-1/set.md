# Set

A **Set** is a data structure that stores **unique elements** and lets you quickly ask:

> “Have I already seen this?”

That’s its entire purpose.

------

## Core properties of a Set

A Set has three defining rules:

1. **No duplicates**
   Each value can appear **at most once**.
2. **Unordered (conceptually)**
   A Set does **not** care about order.
3. **Fast membership check**
   You can efficiently ask:
   `contains(x)` → yes / no

------

## Minimal abstract operations

Language-agnostic, a Set supports:

```
add(x)        // insert element
contains(x)  // membership test
```

That’s it.
No indexing. No traversal order. No position.

------

## Intuition (real-world analogy)

Think of a **guest list** at an event:

- You don’t care *when* someone arrived
- You only care whether their name is **already on the list**
- Writing the same name twice is pointless

That list is a Set.

------

## Visual intuition

![Image](https://www.researchgate.net/publication/254824518/figure/fig4/AS%3A669480120250374%401536627907739/Evolution-of-a-set-data-structure.png)

![Image](https://miro.medium.com/1%2AP_4TlPHhOmFzYIMPJYC3nw.png)

![Image](https://www.researchgate.net/publication/267347887/figure/fig2/AS%3A642853663424512%401530279665128/Sample-fuzzy-set-membership-function.png)

```
Set = { A, B, C }

add(B)  → still { A, B, C }
add(D)  → { A, B, C, D }
```

------

## Why Sets are mandatory for BFS (and your crawler)

In BFS, a Set is used as the **visited set**.

Its job is to prevent this:

```
A → B → C
↑         ↓
└─────────┘
```

Without a Set:

- You revisit the same URLs
- You get infinite loops
- BFS degenerates into chaos

### The key invariant

> **A node is marked “visited” the moment it is enqueued, not when it is processed.**

Why?

- Multiple pages may link to the same URL
- You must enqueue it **once**
- Everyone else must be ignored

------

## In your pipeline (exact mapping)

| Concept                  | Your crawler        |
| ------------------------ | ------------------- |
| Set element              | Canonical URL       |
| Set purpose              | Deduplication       |
| Location                 | Inside `frontier`   |
| Visibility               | Private to frontier |
| Used by scheduler?       | ❌ No                |
| Used by pipeline stages? | ❌ No                |

So conceptually:

```
Frontier state:
  queue   = FIFO queue (BFS order)
  visited = Set<CanonicalURL>
```

------

## What a Set is *not*

| Not a Set        | Why                                      |
| ---------------- | ---------------------------------------- |
| List / array     | Allows duplicates                        |
| Queue            | Enforces order                           |
| Stack            | LIFO semantics                           |
| Map / dictionary | Has key→value meaning (Set is keys-only) |
| Graph            | Stores relationships                     |

A Set answers **one question only**:

> “Have I seen this before?”

------

## One-sentence definition (use this confidently)

> **A Set is a data structure that stores unique elements and supports fast membership checks, used in BFS to ensure each node is visited at most once.**