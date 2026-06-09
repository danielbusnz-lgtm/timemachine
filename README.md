# timemachine

A point-in-time research service in Go. You give it a query and a date, it spawns research agents that gather only information that existed on or before that date, nothing after. Think Firecrawl, but it can only see the past.

The point is to kill lookahead bias. When you backtest a strategy (or an LLM) on a past period, any data that leaked in from the future makes the result a lie. timemachine returns research as it stood on a fixed date, so a backtest at that date is honest.

Early days. The HTTP skeleton runs; the async job engine and the point-in-time fetchers are next.

## Run

```bash
go run ./cmd/gateway
```

Listens on `:8080`.

## Planned API

```
POST /jobs      {query, as_of}   submit a research job, returns a job id
GET  /jobs/{id}                  poll status, then read the result
```

Jobs run async: a worker pool fans out across timestamped sources (Wayback Machine, SEC EDGAR, news archives), filters every result to on-or-before `as_of` in one place, and caches by `(query, as_of)` since the past never changes.

## License

MIT
