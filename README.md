# timemachine

Point-in-time research for AI agents, in Go. Give it a query and a date and it gathers only information that existed on or before that date, nothing after. Firecrawl, but it can only see the past. The point is to kill lookahead bias: backtest a strategy or an LLM on a past period and any future data that leaks in makes the result a lie. timemachine returns research as it stood on a fixed date, so the backtest stays honest. Early days, the HTTP skeleton runs and the async job engine and point-in-time fetchers are next.

## License

MIT
