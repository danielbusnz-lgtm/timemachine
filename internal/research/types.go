// Package research holds the core types, the as-of cutoff filter, and the
// v1 synchronous orchestrator for point-in-time research.
package research

import (
	"context"
	"time"
)

// Query is one research request: what to answer, the moment the answer
// must be limited to, and the candidate URLs to research. v1 takes seed
// URLs from the caller; query-to-URL discovery is a later source concern.
type Query struct {
	Text string
	AsOf time.Time
	URLs []string
}

// Doc is one piece of retrieved content. Timestamp is when the content
// existed (archive capture time, filing date), never when it was fetched.
type Doc struct {
	URL       string
	Timestamp time.Time
	Source    string
	Text      string
}

// Source is one archive backend. Implementations fetch docs for q whose
// content existed at or before asOf. The orchestrator re-filters every
// returned doc through Cutoff regardless: a source's own date handling
// is an optimization, not the guarantee.
type Source interface {
	Name() string
	Fetch(ctx context.Context, q Query, asOf time.Time) ([]Doc, error)
}
