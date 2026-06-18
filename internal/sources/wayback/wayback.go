// Package wayback implements research.Source against the Internet
// Archive's Wayback Machine: a CDX lookup to find the latest snapshot at
// or before the as-of date, then a memento fetch for the content as it
// existed then.
package wayback

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/danielbusnz/timemachine/internal/research"
)

const (
	cdxEndpoint = "https://web.archive.org/cdx/search/cdx"
	// id_ serves the original captured bytes without the Wayback toolbar
	// or rewritten links.
	mementoFmt = "https://web.archive.org/web/%sid_/%s"
	// cdxStamp is the compact timestamp format CDX speaks on both sides.
	cdxStamp = "20060102150405"
	// maxDocChars bounds each doc before ranking so one huge page can't
	// drown the synthesis prompt.
	maxDocChars = 20000
	// minInterval spaces requests so we stay polite to the archive. One
	// URL costs two requests (CDX + memento).
	minInterval = time.Second
)

type Client struct {
	http *http.Client

	mu   sync.Mutex
	last time.Time
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Name() string { return "wayback" }

// Fetch resolves each seed URL to its latest snapshot at or before asOf
// and returns the captured content. A URL with no snapshot or a failed
// fetch is skipped, partial results beat a dead job. It returns an error
// only when nothing could be fetched at all and at least one URL failed.
func (c *Client) Fetch(ctx context.Context, q research.Query, asOf time.Time) ([]research.Doc, error) {
	var docs []research.Doc
	var lastErr error
	for _, seed := range q.URLs {
		doc, err := c.fetchOne(ctx, seed, asOf)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", seed, err)
			continue
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return docs, nil
}

func (c *Client) fetchOne(ctx context.Context, seed string, asOf time.Time) (research.Doc, error) {
	stamp, original, err := c.latestSnapshot(ctx, seed, asOf)
	if err != nil {
		return research.Doc{}, err
	}

	captured, err := time.ParseInLocation(cdxStamp, stamp, time.UTC)
	if err != nil {
		return research.Doc{}, fmt.Errorf("bad CDX timestamp %q: %w", stamp, err)
	}

	body, err := c.get(ctx, fmt.Sprintf(mementoFmt, stamp, original))
	if err != nil {
		return research.Doc{}, err
	}

	text := stripHTML(string(body))
	if len(text) > maxDocChars {
		text = text[:maxDocChars]
	}

	return research.Doc{
		URL:       original,
		Timestamp: captured,
		Source:    c.Name(),
		Text:      text,
	}, nil
}

// latestSnapshot asks CDX for the most recent 200 capture at or before
// asOf and returns its timestamp and original URL.
func (c *Client) latestSnapshot(ctx context.Context, seed string, asOf time.Time) (stamp, original string, err error) {
	params := url.Values{
		"url":    {seed},
		"output": {"json"},
		"to":     {asOf.UTC().Format(cdxStamp)},
		"filter": {"statuscode:200"},
		// Negative limit means "from the end": the single most recent
		// capture within the to-bound, no client-side scanning.
		"limit":      {"-1"},
		"fastLatest": {"true"},
	}

	body, err := c.get(ctx, cdxEndpoint+"?"+params.Encode())
	if err != nil {
		return "", "", err
	}

	// CDX JSON is rows of strings; row 0 is the header
	// [urlkey, timestamp, original, mimetype, statuscode, digest, length].
	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return "", "", fmt.Errorf("CDX response: %w", err)
	}
	if len(rows) < 2 || len(rows[1]) < 3 {
		return "", "", fmt.Errorf("no snapshot at or before %s", asOf.Format(time.DateOnly))
	}
	return rows[1][1], rows[1][2], nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	c.throttle()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "timemachine/0.1 (point-in-time research; github.com/danielbusnz/timemachine)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// throttle enforces minInterval between outbound requests so a multi-URL
// job stays polite to the archive.
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := minInterval - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

var (
	scriptRe = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// stripHTML is the v1 text extractor: drop script/style blocks, drop
// tags, collapse whitespace. Crude but dependency-free; proper readability
// extraction is a later concern.
func stripHTML(s string) string {
	s = scriptRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
