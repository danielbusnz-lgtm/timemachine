package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/danielbusnz-lgtm/timemachine/internal/cache"
	"github.com/danielbusnz-lgtm/timemachine/internal/middleware"
	"github.com/danielbusnz-lgtm/timemachine/internal/research"
	"github.com/danielbusnz-lgtm/timemachine/internal/sources/wayback"
	"github.com/danielbusnz-lgtm/timemachine/internal/synth"
)

// researchRequest is the v1 wire format. as_of takes "2021-03-15" or full
// RFC 3339; a bare date means end of that day UTC, the natural reading of
// "as of March 15th".
type researchRequest struct {
	Query string   `json:"query"`
	AsOf  string   `json:"as_of"`
	URLs  []string `json:"urls"`
}

type sourceOut struct {
	URL      string `json:"url"`
	Captured string `json:"captured"`
	Source   string `json:"source"`
}

type researchResponse struct {
	Answer  string      `json:"answer,omitempty"`
	Sources []sourceOut `json:"sources"`
	Cached  bool        `json:"cached"`
}

const (
	maxSeedURLs = 10
	jobTimeout  = 3 * time.Minute
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sources := []research.Source{wayback.New()}

	var synthesizer research.Synthesizer
	switch {
	case os.Getenv("OPENAI_API_KEY") != "":
		synthesizer = synth.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		synthesizer = synth.NewAnthropic(os.Getenv("ANTHROPIC_API_KEY"))
	default:
		log.Warn("no OPENAI_API_KEY or ANTHROPIC_API_KEY: /research returns sources without a synthesized answer")
	}

	store := cache.NewMemory()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // health check, nothing to do on error
	})

	mux.HandleFunc("POST /research", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		var req researchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		q, err := parseRequest(req)
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}

		key := cache.Key(q)
		if result, ok := store.Get(key); ok {
			writeResult(w, result, true)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), jobTimeout)
		defer cancel()

		result, err := research.Run(ctx, q, sources, synthesizer)
		if err != nil {
			log.Error("research job failed", "query", q.Text, "as_of", q.AsOf, "err", err)
			httpError(w, http.StatusBadGateway, err.Error())
			return
		}

		store.Set(key, result)
		writeResult(w, result, false)
	})

	addr := ":8080"
	log.Info("timemachine gateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, middleware.Logging(mux)); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func parseRequest(req researchRequest) (research.Query, error) {
	if req.Query == "" {
		return research.Query{}, errors.New("query is required")
	}
	if len(req.URLs) == 0 {
		return research.Query{}, errors.New("urls is required: v1 researches caller-supplied seed URLs")
	}
	if len(req.URLs) > maxSeedURLs {
		return research.Query{}, errors.New("too many urls: v1 caps at 10 per job")
	}

	asOf, err := parseAsOf(req.AsOf)
	if err != nil {
		return research.Query{}, err
	}
	if asOf.After(time.Now()) {
		return research.Query{}, errors.New("as_of is in the future: timemachine only goes backward")
	}

	return research.Query{Text: req.Query, AsOf: asOf, URLs: req.URLs}, nil
}

func parseAsOf(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("as_of is required (\"2021-03-15\" or RFC 3339)")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.Parse(time.DateOnly, s); err == nil {
		// End of that day: "as of March 15th" includes March 15th.
		return d.Add(24*time.Hour - time.Nanosecond), nil
	}
	return time.Time{}, errors.New("as_of must be \"2006-01-02\" or RFC 3339")
}

func writeResult(w http.ResponseWriter, result research.Result, cached bool) {
	out := researchResponse{Answer: result.Answer, Cached: cached, Sources: make([]sourceOut, 0, len(result.Docs))}
	for _, d := range result.Docs {
		out.Sources = append(out.Sources, sourceOut{
			URL:      d.URL,
			Captured: d.Timestamp.UTC().Format(time.RFC3339),
			Source:   d.Source,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out) // encode failure means the client hung up
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
