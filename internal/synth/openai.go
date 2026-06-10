// Package synth makes the one LLM call at the end of a research job:
// question plus date-filtered sources in, cited answer out.
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
	endpoint = "https://api.openai.com/v1/chat/completions"
	model    = "gpt-4o-mini"
	// maxPromptCharsPerDoc bounds each doc inside the prompt; docs are
	// already capped at fetch time, this is the tighter prompt budget.
	maxPromptCharsPerDoc = 6000
)

type OpenAI struct {
	apiKey string
	http   *http.Client
}

func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{apiKey: apiKey, http: &http.Client{Timeout: 120 * time.Second}}
}

// Synthesize makes the single LLM call. The system prompt pins the model
// to the as-of date and to the provided sources; the date filtering
// itself already happened in research.Cutoff, this is belt over the
// braces for the model's own background knowledge.
func (o *OpenAI) Synthesize(ctx context.Context, q research.Query, docs []research.Doc) (string, error) {
	reqBody, err := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt(q)},
			{"role": "user", "content": userPrompt(q, docs)},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("openai: decode: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai: empty choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
