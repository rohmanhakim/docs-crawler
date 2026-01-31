### BFS (Breadth-First Search) — in Task 1.2 context

**BFS (Breadth-First Search)** is a traversal strategy where you explore **all nodes at the current depth before moving deeper**.

In a crawler, that means:

> **Visit all pages at depth \*d\* before visiting any page at depth \*d+1\*.**

------

## Core idea (plain terms)

Starting from seed URLs:

1. Crawl the seed page(s) (depth = 0)
2. Collect all links found on those pages → depth = 1
3. Crawl **all** depth-1 pages
4. Collect their links → depth = 2
5. Repeat until limits are hit (max depth, max pages, etc.)

You never “go deep early”.

------

## Visual intuition

![Image](https://miro.medium.com/v2/resize%3Afit%3A1400/1%2AVM84VPcCQe0gSy44l9S5yA.jpeg)

![Image](https://deen3evddmddt.cloudfront.net/uploads/content-images/bfs.webp)

![Image](https://www.btechsmartclass.com/data_structures/ds_images/BFS.jpg)

Think in **layers**:

```
Depth 0:  A
Depth 1:  B   C   D
Depth 2:  E F  G   H I
```

BFS order:

```
A → B → C → D → E → F → G → H → I
```

------

## Why BFS is explicitly required in your crawler design

From your **design + technical documents**, BFS is a **non-negotiable invariant**:

> *Frontier Responsibilities: Maintain BFS ordering, track crawl depth, prevent infinite traversal.*

### BFS guarantees you rely on:

1. **Deterministic ordering**
   Same seed + same site → same crawl order → same output
2. **Correct depth semantics**
   `maxDepth = N` has a precise meaning
3. **Fair coverage**
   You don’t disappear into a deep subsection while ignoring sibling docs
4. **Good RAG structure**
   Higher-level docs (indexes, guides) are processed before deep API leaves

------

## Where BFS lives in *your* architecture

**Important separation of concerns:**

| Component           | Role with respect to BFS                    |
| ------------------- | ------------------------------------------- |
| **Scheduler**       | Decides *whether* a URL may enter the crawl |
| **Frontier**        | Decides *when* a URL is crawled (BFS order) |
| **Pipeline stages** | Completely unaware of BFS                   |

In your codebase:

- `frontier.Frontier`
  - Owns BFS queueing
  - Tracks depth
  - Deduplicates URLs
- `frontier.CrawlToken`
  - Carries the URL + depth
- `scheduler.SubmitUrlForAdmission()`
  - Must **not** implement BFS
  - Only performs semantic admission (robots, scope, limits)

------

## What BFS is **not** (common pitfalls)

❌ Not “follow links immediately as you find them”
❌ Not DFS (depth-first)
❌ Not priority-based by content type
❌ Not influenced by fetch success/failure
❌ Not affected by metadata or errors

If a page fails to fetch, BFS **ordering does not change**.

------

## Minimal mental model for implementation

You should be able to say:

> “The frontier behaves like a FIFO queue segmented by depth.”

Conceptually:

```
Queue:
[ (url=A, depth=0) ]

→ dequeue A
→ enqueue children at depth=1

Queue:
[ (B,1), (C,1), (D,1) ]
```

That’s BFS. Nothing more. Nothing less.

------

## One-sentence definition (use this in reviews)

> **BFS is a crawl strategy where URLs are dequeued strictly in increasing depth order, ensuring that all pages at depth \*d\* are processed before any page at depth \*d+1\*.**