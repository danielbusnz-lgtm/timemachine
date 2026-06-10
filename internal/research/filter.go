package research

import "time"

// Cutoff is the one chokepoint that makes the product honest: nothing
// with a Timestamp after asOf survives. Docs with a zero Timestamp are
// dropped too: content that cannot prove when it existed has no place in
// a point-in-time answer. Every doc the orchestrator uses passes through
// here, and only here, so the guarantee stays provable.
func Cutoff(docs []Doc, asOf time.Time) []Doc {
	kept := make([]Doc, 0, len(docs))
	for _, d := range docs {
		if d.Timestamp.IsZero() || d.Timestamp.After(asOf) {
			continue
		}
		kept = append(kept, d)
	}
	return kept
}
