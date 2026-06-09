# timemachine: design

Here's how I'd build it. The whole design follows one principle: the orchestrator is plain Go, the only LLM call is the last step, and date-filtering happens in exactly one place.

**The API (async, because fan-out is slow):**

```
POST /research   {query, as_of}   -> returns {job_id}
GET  /research/{job_id}            -> {status, result?}
```

**The shape:**

```
POST /research ──> [cache: redis key hash(query)+as_of] ──HIT──> return
                              │ MISS
                              v
                   [enqueue job (asynq) ──> return job_id]
                              │
                              v
        ┌──────────── Orchestrator worker (plain Go) ────────────┐
        │  1. derive per-source subqueries                       │
        │  2. errgroup: fan out to sources concurrently          │
        │       ├── wayback   (CDX + memento)                    │
        │       ├── edgar     (full-text + filing dates)         │
        │       └── news      (archive API)                      │
        │  3. EACH source filters its docs to timestamp <= as_of │
        │  4. merge, dedup by normalized URL                     │
        │  5. rank (date proximity + BM25), take top-K chunks    │
        │  6. assemble citation prompt, ONE LLM synthesis call   │
        │  7. write result to redis (+ object store for raw)     │
        └────────────────────────────────────────────────────────┘
```

**The one interface that keeps it clean:**

```go
type Source interface {
    Name() string
    Fetch(ctx context.Context, q Query, asOf time.Time) ([]Doc, error)
}

type Doc struct {
    URL       string
    Timestamp time.Time   // when this content existed
    Source    string
    Text      string
    // ...
}
```

Every source is just an implementation of this. Adding a new archive = one new `Source`. The orchestrator never knows the details, it ranges over a `[]Source` and runs them in parallel.

**The concurrency core (the Go part, and the whole point):**

• Per-job fan-out with `errgroup.WithContext`: launch every source as a goroutine, each wrapped in `context.WithTimeout`. Collect what returns; a slow or failing source degrades gracefully (partial results) instead of killing the job.
• First-success racing where two endpoints serve the same thing (Wayback CDX vs direct memento): `select` over channels, take the first non-empty.
• Per-source rate limits with `golang.org/x/time/rate`, one limiter per source so you respect Wayback/EDGAR politely without throttling the others.
• Worker pool = asynq concurrency setting. Horizontal scale = run more worker binaries draining the same Redis queue.

**The date filter is the product, so it gets one chokepoint.** Every `Doc` carries a `Timestamp`. Nothing with `Timestamp > asOf` survives, enforced in one function the orchestrator calls. That single line is the "honest backtest" guarantee. If it lives in one place, you can prove it; if it's scattered across sources, you can't. (This mirrors Firecrawl's "filter in one place" lesson.)

**Caching: the past is immutable, so cache forever.** A `(query, as_of)` result never needs invalidation, the world on 2021-03-15 won't change. Redis hot, optional object store for raw payloads. Secondary `(url, as_of-bucket)` cache so docs shared across queries aren't refetched.

**Storage / deploy:** Redis (cache + queue + job state + dedup sets), optional S3/GCS for raw docs, one Go binary (or split API and worker binaries). Deploys on Fly.io / Railway / a VM. Stateless workers, so scaling is trivial.

**Where I'd actually start (don't build all of it at once):**

• v1: API + one source (Wayback only) + the date filter + a cache + one LLM synthesis call. Synchronous, no queue yet. Get the honest "research as of date X" loop working end to end with a single source.
• v2: add the `asynq` queue + worker pool + the `errgroup` fan-out, and add EDGAR as the second source. Now it's genuinely concurrent.
• v3: news archives, ranking/chunking polish, the object-store cold cache, rate limiters.

That sequencing keeps it shippable at every step, and v1 alone already demos the core idea.
