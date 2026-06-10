package synth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/danielbusnz-lgtm/timemachine/internal/research"
)

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
	// Haiku is the fast, cheap tier; synthesis is one bounded call per job
	// and the prompt is small (top-K docs, each capped), so the premium
	// tiers buy nothing here.
	anthropicModel = "claude-haiku-4-5"
	// Anthropic requires max_tokens. Cited summaries fit comfortably; raise
	// if jobs start returning visibly truncated answers.
	anthropicMaxTokens = 1024
)

// Anthropic synthesizes with Claude over the raw Messages API. Same shape
// as the OpenAI synthesizer so either can sit behind research.Synthesizer.
type Anthropic struct {
	apiKey string
	http   *http.Client
}

func NewAnthropic(apiKey string) *Anthropic {
	return &Anthropic{apiKey: apiKey, http: &http.Client{Timeout: 120 * time.Second}}
}

func (a *Anthropic) Synthesize(ctx context.Context, q research.Query, docs []research.Doc) (string, error) {
	reqBody, err := json.Marshal(map[string]any{
		"model":      anthropicModel,
		"max_tokens": anthropicMaxTokens,
		"system":     systemPrompt(q),
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt(q, docs)},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	// Content is a list of typed blocks; the answer is the text blocks
	// concatenated.
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}
	var out string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			out += block.Text
		}
	}
	if out == "" {
		return "", fmt.Errorf("anthropic: no text content in response")
	}
	return out, nil
}
