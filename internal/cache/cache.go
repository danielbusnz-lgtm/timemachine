// Package cache stores finished research results. The past is immutable,
// so a (query, asOf, urls) result never needs invalidation: cache forever.
// v1 is in-process memory behind a small interface so Redis can take the
// seat in v2 without touching callers.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielbusnz-lgtm/timemachine/internal/research"
)

type Store interface {
	Get(key string) (research.Result, bool)
	Set(key string, r research.Result)
}

// Key derives the immutable identity of a job: the question, the as-of
// instant, and the seed URLs (order-insensitive).
func Key(q research.Query) string {
	urls := append([]string(nil), q.URLs...)
	sort.Strings(urls)
	h := sha256.New()
	h.Write([]byte(q.Text))
	h.Write([]byte{0})
	h.Write([]byte(q.AsOf.UTC().Format(time.RFC3339)))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(urls, "\n")))
	return hex.EncodeToString(h.Sum(nil))
}

type Memory struct {
	mu sync.RWMutex
	m  map[string]research.Result
}

func NewMemory() *Memory {
	return &Memory{m: make(map[string]research.Result)}
}

func (c *Memory) Get(key string) (research.Result, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.m[key]
	return r, ok
}

func (c *Memory) Set(key string, r research.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = r
}
