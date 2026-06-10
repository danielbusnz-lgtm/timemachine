package research

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Result of one research job: the synthesized answer and the docs it was
// built from. Answer is empty when no Synthesizer was configured.
type Result struct {
	Answer string
	Docs   []Doc
}

// Synthesizer turns the question plus the surviving docs into a cited
// answer. The orchestrator makes exactly one call to it, last.
type Synthesizer interface {
	Synthesize(ctx context.Context, q Query, docs []Doc) (string, error)
}

// topK bounds how many docs reach the synthesis prompt.
const topK = 8

// Run executes one synchronous v1 job: fetch from every source, enforce
// the cutoff, dedup, rank by proximity to asOf, take top-K, then one
// synthesis call. A failing source degrades to partial results; it only
// errors the job when nothing survives at all.
func Run(ctx context.Context, q Query, sources []Source, synth Synthesizer) (Result, error) {
	var all []Doc
	var lastErr error
	for _, s := range sources {
		docs, err := s.Fetch(ctx, q, q.AsOf)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", s.Name(), err)
			continue
		}
		all = append(all, docs...)
	}

	// The guarantee. Sources filter as an optimization; this is the proof.
	all = Cutoff(all, q.AsOf)
	all = dedup(all)

	if len(all) == 0 {
		if lastErr != nil {
			return Result{}, lastErr
		}
		return Result{}, errors.New("no documents survived the as-of cutoff")
	}

	// Closest to the as-of moment first: the freshest view of the world
	// that was still legal to see.
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})
	if len(all) > topK {
		all = all[:topK]
	}

	if synth == nil {
		return Result{Docs: all}, nil
	}
	answer, err := synth.Synthesize(ctx, q, all)
	if err != nil {
		return Result{}, fmt.Errorf("synthesis: %w", err)
	}
	return Result{Answer: answer, Docs: all}, nil
}

// dedup drops docs whose normalized URL was already seen, keeping the
// first occurrence (sources are ordered, earlier sources win).
func dedup(docs []Doc) []Doc {
	seen := make(map[string]struct{}, len(docs))
	kept := docs[:0]
	for _, d := range docs {
		key := strings.TrimRight(strings.ToLower(d.URL), "/")
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		kept = append(kept, d)
	}
	return kept
}
