# FIFO Queue

A **FIFO queue** is a data structure where **the first item added is the first item removed**.

FIFO = **First In, First Out**.

------

## Intuition (real-world analogy)

Think of a **line at a checkout counter**:

- First person in line → first person served
- New people join at the back
- Nobody cuts in front

That is exactly how a FIFO queue behaves.

------

## Core operations (abstract, not language-specific)

A FIFO queue supports two essential operations:

1. **Enqueue** — add an item to the *back* of the queue
2. **Dequeue** — remove an item from the *front* of the queue

Visually:

![Image](https://www.researchgate.net/publication/335465510/figure/fig3/AS%3A797185583095808%401567075262321/FIFO-Mechanism-shows-how-a-FIFO-queue-operates-The-multiplexer-puts-the-incoming-flows.png)

![Image](https://he-s3.s3.amazonaws.com/media/uploads/cf1e1c1.png)

![Image](https://images.openai.com/static-rsc-3/F1TMjYk1iqbiTpeWLJcUiLAPynlPTN7iuvYmUBxEeGgMcjUzyiv6FTNjVXK7LjKkhuPGFsP5SbC4TsALIKHpFONUyCwFL4zfqfMypes52iY?purpose=fullsize)

```
enqueue → [ A ][ B ][ C ] → dequeue
            ↑ front        ↑ back
```

If you enqueue in this order:

```
enqueue(A)
enqueue(B)
enqueue(C)
```

You must dequeue in this order:

```
A → B → C
```

------

## Why FIFO matters for BFS (and this crawler)

BFS **depends** on FIFO behavior.

### Key rule:

> **URLs are processed in the exact order they are discovered.**

Because:

- URLs discovered at depth `d` are enqueued **before**
- URLs discovered at depth `d+1`

FIFO guarantees:

- All depth-`d` URLs are dequeued before any depth-`d+1` URLs
- No “jumping ahead” into deeper pages

That’s how BFS preserves **layer-by-layer traversal**.

------

## FIFO vs other structures (important contrast)

| Structure      | Order                  | Used for              |
| -------------- | ---------------------- | --------------------- |
| **FIFO Queue** | First in → first out   | BFS                   |
| Stack (LIFO)   | Last in → first out    | DFS                   |
| Priority Queue | Highest priority first | Best-first / Dijkstra |

If you replace FIFO with:

- a **stack** → you get DFS (wrong for your design)
- a **priority queue** → you lose deterministic BFS ordering

------

## In this project’s terms

Inside `frontier.Frontier` (conceptually):

```
Queue item = (URL, depth)
```

Rules:

- Enqueue discovered URLs at the back
- Dequeue the oldest admitted URL from the front
- Never reorder
- Never skip ahead

The **scheduler** decides *what may enter* the queue
The **frontier** decides *when it is processed* (FIFO)

------

## One-sentence definition (use this)

> **A FIFO queue is a data structure where elements are processed in the same order they are added: first in, first out.**