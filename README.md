# timemachine

[![Go version](https://img.shields.io/badge/go-1.26-00ADD8?style=flat-square&logo=go)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

Point-in-time research API. Give it a query and a date; it returns only what was knowable on or before that date.

The problem it solves: lookahead bias. Backtesting a trading strategy or grading an LLM on historical data is invalid the moment any post-cutoff information touches the result. timemachine enforces a hard date boundary at the query level, not as an afterthought. Every document carries a timestamp; nothing with `timestamp > as_of` survives the cutoff filter.

## Status

v1 works end to end, synchronously: `POST /research` fetches caller-supplied URLs from the Wayback Machine as they existed on the as-of date, filters every document through the cutoff, synthesizes one answer with a single LLM call, and caches the result. The past is immutable, so cache entries never expire.

Not built yet: the async job queue, additional sources (EDGAR, news archives), and ranking. See [DESIGN.md](DESIGN.md) for the architecture and build order.

## Quickstart

Requires Go 1.26+ and one LLM key for the synthesis step.

```bash
git clone https://github.com/danielbusnz-lgtm/timemachine
cd timemachine
OPENAI_API_KEY=sk-... go run ./cmd/gateway   # or ANTHROPIC_API_KEY
```

Server listens on `:8080`.

```bash
curl -s http://localhost:8080/research \
  -d '{
    "query": "what was the public sentiment on GameStop stock",
    "as_of": "2021-01-15",
    "urls": ["https://www.reddit.com/r/wallstreetbets/", "https://www.bloomberg.com/"]
  }'
```

```json
{
  "answer": "...synthesized from sources as they stood on 2021-01-15...",
  "cached": false,
  "sources": [
    { "url": "...", "captured": "2021-01-14T22:10:03Z", "source": "wayback" }
  ]
}
```

A bare `as_of` date means end of that day UTC, the natural reading of "as of January 15th". RFC 3339 timestamps work too. v1 researches up to 10 caller-supplied seed URLs per job; query-to-URL discovery comes with later sources.

## Architecture

One principle: plain Go orchestrator, one LLM call at the end, date filtering in exactly one function.

```
POST /research ──> [cache: hash(query, as_of, urls)] ──HIT──> return
                          │ MISS
                          v
    ┌──────────────── Orchestrator ─────────────────┐
    │  1. fan out to sources                        │
    │       └── wayback   (CDX + snapshot fetch)    │
    │  2. every doc re-filtered: timestamp <= as_of │
    │  3. merge, dedup by normalized URL            │
    │  4. one LLM synthesis call                    │
    │  5. cache the result, forever                 │
    └────────────────────────────────────────────────┘
```

The `Source` interface is the extension point. Adding an archive = one new implementation of `Fetch(ctx, query, asOf) ([]Doc, error)`. Sources may do their own date handling as an optimization, but the orchestrator re-filters every returned document through the cutoff regardless. The guarantee lives in one function.

## Roadmap

- **v1** (done): synchronous research loop, Wayback source, cutoff filter, in-memory cache, one synthesis call.
- **v2**: async job queue (`POST /research` returns a job ID), worker pool, errgroup fan-out, EDGAR as second source, Redis cache.
- **v3**: news archives, BM25 ranking, object-store cold cache, per-source rate limits.

## License

MIT. See [LICENSE](LICENSE).
