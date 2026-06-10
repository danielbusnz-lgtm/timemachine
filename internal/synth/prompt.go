package synth

import (
	"fmt"
	"strings"
	"time"

	"github.com/danielbusnz-lgtm/timemachine/internal/research"
)

// systemPrompt pins the model to the as-of date and to the provided
// sources. The date filtering already happened in research.Cutoff; this
// is belt over the braces for the model's own background knowledge.
func systemPrompt(q research.Query) string {
	asOf := q.AsOf.UTC().Format("January 2, 2006")
	return fmt.Sprintf(
		"You are a research assistant whose knowledge is frozen at %s. "+
			"Answer the question using ONLY the numbered sources provided, all of which "+
			"existed on or before that date. Cite sources inline as [n]. Write as if %s "+
			"is today: never mention or rely on anything that happened after it, including "+
			"anything you might know from training. If the sources are insufficient to "+
			"answer, say exactly what is missing.",
		asOf, asOf,
	)
}

// userPrompt assembles the question and the numbered, date-stamped
// sources, with each doc capped so one page can't drown the prompt.
func userPrompt(q research.Query, docs []research.Doc) string {
	asOf := q.AsOf.UTC().Format("January 2, 2006")
	var b strings.Builder
	fmt.Fprintf(&b, "Question (answer as of %s): %s\n\nSources:\n", asOf, q.Text)
	for i, d := range docs {
		text := d.Text
		if len(text) > maxPromptCharsPerDoc {
			text = text[:maxPromptCharsPerDoc]
		}
		fmt.Fprintf(&b, "\n[%d] %s (captured %s)\n%s\n",
			i+1, d.URL, d.Timestamp.UTC().Format(time.DateOnly), text)
	}
	return b.String()
}
