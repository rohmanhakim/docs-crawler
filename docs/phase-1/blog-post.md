# The Crawler's Dilemma: When "Standard BFS" Hits Production

## Introduction

When building a web crawler, one of the first algorithms we reach for is Breadth-First Search (BFS). It's simple, intuitive, and theoretically guarantees we'll explore all pages at depth *d* before diving deeper. But as I recently discovered while building a documentation crawler, **textbook BFS and production BFS are two very different beasts**.

This post explores the architectural tension between theoretical purity and pragmatic engineering—and how we navigated it.

## The Problem: What Does "BFS" Actually Mean?

Our crawler needed to satisfy a deceptively simple requirement: **maintain BFS ordering**. At first glance, this seems obvious—use a FIFO queue, enqueue URLs as we discover them, dequeue them in order. Right?

Not quite.

Consider this scenario:

```
We start crawling at the homepage (depth 0).
The homepage links to two pages: About (depth 1) and Contact (depth 1).
We crawl About and discover it links to Team (depth 2).
We add Team to our queue.

But wait—the Contact page ALSO links to a Careers page (depth 1),
which we discover AFTER Team is already in the queue.

Our queue now looks like: [Team (depth 2), Careers (depth 1)]
```

Here's the dilemma: **Do we process Team (depth 2) first because it was discovered earlier? Or do we "jump" Careers (depth 1) ahead because it's at a shallower depth?**

### The Theorist vs. The Practitioner

This scenario sparked an internal debate that I suspect many engineering teams face:

**The Theorist's Argument:**  
"Strict BFS means ALL depth-1 pages must be exhausted before ANY depth-2 page. If we discover Careers late, we must prioritize it over Team. Otherwise, we're violating BFS."

**The Practitioner's Counter:**  
"But that requires either: (a) reordering our queue dynamically, or (b) using a priority queue by depth. Both add complexity. And if the site changes mid-crawl, we might never see that 'late' depth-1 page anyway."

## The Options on the Table

We identified three architectural approaches, each with distinct tradeoffs:

### Option 1: Single FIFO Queue (The "Discovery Order" Approach)

**Implementation:** One queue. URLs are dequeued in discovery order.

```go
type Frontier struct {
    queue []URL  // Simple FIFO
}
```

**Benefits:**
- Simple to implement and reason about
- O(1) enqueue/dequeue
- Predictable memory layout

**Tradeoffs:**
- Violates strict BFS if shallow URLs are discovered late
- Depth limits become fuzzy: `maxDepth=2` doesn't guarantee all depth-2 pages are captured
- Determinism suffers: network timing affects crawl order

**Best For:** Quick prototypes, scenarios where "good enough" ordering suffices.

### Option 2: Priority Queue by Depth (The "Strict BFS" Approach)

**Implementation:** Min-heap ordered by depth.

```go
type Frontier struct {
    queue *PriorityQueue  // Ordered by depth
}
```

**Benefits:**
- Guarantees strict depth ordering
- Late-discovered shallow URLs automatically "bubble up"

**Tradeoffs:**
- O(log n) operations vs O(1) for FIFO
- Non-deterministic within same depth (heap property doesn't preserve FIFO)
- Overkill if you don't actually need strict global ordering

**Best For:** Search engines, scenarios where depth priority is critical.

### Option 3: Layered BFS (Per-Depth FIFO Queues)

**Implementation:** Separate FIFO queue for each depth level.

```go
type Frontier struct {
    queuesByDepth map[int][]URL  // One queue per depth
    currentDepth  int
}
```

**Benefits:**
- Maintains strict BFS: all depth-N exhausted before depth-N+1
- O(1) operations per queue
- FIFO ordering preserved *within* each depth
- Natural checkpointing: we know exactly when each layer is complete

**Tradeoffs:**
- More complex structure (map of queues vs single queue)
- Must handle nil queues when depth levels are sparse
- Slightly higher memory overhead

**Best For:** Archival crawlers, compliance tools, scenarios requiring deterministic, auditable behavior.

## Our Decision

We chose **Option 3: Layered BFS**.

Our crawler is designed for documentation archival, where:
- **Determinism matters:** Re-running the crawl should produce identical output
- **Completeness matters:** We need guaranteed snapshots up to specific depths
- **Auditability matters:** We must be able to say "layer 2 is complete"

The layered approach gives us these guarantees while preserving O(1) performance and clear semantics.

## Implementation Insights

### The Core Algorithm

```go
func (f *Frontier) Dequeue() (URL, bool) {
    // Always exhaust current depth before advancing
    for depth := 0; depth <= f.maxDepth; depth++ {
        if queue := f.queuesByDepth[depth]; queue != nil && len(queue) > 0 {
            return queue.dequeue(), true
        }
    }
    return URL{}, false
}
```

### The Subtle Bugs

Our journey wasn't without pitfalls:

**Bug 1: Nil Pointer Dereference**  
When we submitted a URL at depth 2 before any URL at depth 1, `queuesByDepth[1]` was nil. Our first implementation panicked when checking `queue.Size()`.

**Lesson:** Always nil-check before accessing mapped queues.

**Bug 2: Race Conditions**  
Multiple worker goroutines calling `Submit()` and `Dequeue()` simultaneously caused data races.

**Lesson:** Even "simple" data structures need synchronization in concurrent environments. We added a mutex:

```go
type Frontier struct {
    mu            sync.Mutex
    queuesByDepth map[int][]URL
    // ...
}
```

**Bug 3: Deduplication Edge Cases**  
Using `map[url.URL]struct{}` for deduplication failed because `url.URL` contains pointer fields. Two semantically identical URLs could have different memory addresses.

**Lesson:** Use canonicalized URL strings for map keys, not URL structs.

### Testing Strategy

We wrote tests that specifically target the BFS invariant:

```go
func TestFrontier_LateDiscovery(t *testing.T) {
    // Submit depth-2 URL first
    frontier.Submit(urlC, Depth(2))
    
    // Then submit depth-1 URL
    frontier.Submit(urlD, Depth(1))
    
    // Depth-1 MUST be dequeued first, despite being submitted second
    token, _ := frontier.Dequeue()
    assert.Equal(t, 1, token.Depth())
}
```

We also run with Go's race detector (`go test -race`) to catch concurrency issues.

## Key Lessons for Readers

### 1. Clarify Requirements Early

"BFS ordering" means different things to different people. Is strict depth ordering required, or is FIFO-with-depth-tracking sufficient? Get explicit about invariants.

### 2. Match Architecture to Goals

- Building a search engine? Priority queue might be right.
- Archiving documentation? Layered BFS provides the guarantees you need.
- Quick prototype? Single FIFO queue is fine.

### 3. The "Node D Problem" Is Real

In dynamic graphs, late-discovered shallow nodes are inevitable. Your architecture must either:
- Accept that you'll miss them (standard FIFO)
- Reorder to accommodate them (priority queue)
- Queue them in their correct layer (layered BFS)

### 4. Concurrency Isn't an Afterthought

Even simple crawlers become concurrent. Design for thread-safety from the start, not as a retrofit.

### 5. Test the Invariants, Not the Implementation

Don't just test that your code runs—test that your guarantees hold. If you claim BFS ordering, write a test that would fail if BFS were violated.

## Conclusion

The "Crawler's Dilemma" taught us that algorithmic purity and production pragmatism require careful balancing. By understanding the tradeoffs between FIFO, priority queues, and layered BFS, we chose an architecture that matches our specific needs: deterministic, auditable, archival crawling.

The layered BFS approach—canal locks rather than a single bucket—gives us the structural guarantees we need without sacrificing performance. Sometimes, the best solution isn't the simplest or the most theoretically pure, but the one that directly addresses your actual requirements.

---

*What crawler architectures have you explored? I'd love to hear about your own tradeoffs and decisions in the comments.*
