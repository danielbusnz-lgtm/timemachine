# timemachine

[![CI](https://github.com/danielbusnz/timemachine/actions/workflows/ci.yml/badge.svg)](https://github.com/danielbusnz/timemachine/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/danielbusnz/timemachine)](https://goreportcard.com/report/github.com/danielbusnz/timemachine)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

Point-in-time research API. Give it a query and a date; it returns only what was knowable on or before that date.

The problem it solves: lookahead bias. Backtesting a trading strategy or grading an LLM on historical data is invalid the moment any post-cutoff information touches the result. timemachine enforces a hard date boundary at the query level, not as an afterthought. Every document carries a timestamp; nothing with `timestamp > as_of` survives the cutoff filter.

## Status

v1 works end to end, synchronously: `POST /research` fetches caller-supplied URLs from the Wayback Machine as they existed on the as-of date, filters every document through the cutoff, synthesizes one answer with a single LLM call, and caches the result. The past is immutable, so cache entries never expire.

Not built yet: the async job queue, additional sources (EDGAR, news archives), and ranking. See [DESIGN.md](DESIGN.md) for the architecture and build order.

## Quickstart

Requires Go 1.26+ and one LLM key for the synthesis step.

```bash
git clone https://github.com/danielbusnz/timemachine
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

## License

MIT. See [LICENSE](LICENSE).
