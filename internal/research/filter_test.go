package research

import (
	"testing"
	"time"
)

func TestCutoff(t *testing.T) {
	asOf := time.Date(2021, 3, 15, 0, 0, 0, 0, time.UTC)
	before := asOf.Add(-24 * time.Hour)
	after := asOf.Add(time.Second)

	docs := []Doc{
		{URL: "a", Timestamp: before},
		{URL: "b", Timestamp: asOf},   // boundary: exactly asOf survives
		{URL: "c", Timestamp: after},  // one second late dies
		{URL: "d"},                    // zero timestamp dies
		{URL: "e", Timestamp: before}, // order among survivors is preserved
	}

	got := Cutoff(docs, asOf)

	want := []string{"a", "b", "e"}
	if len(got) != len(want) {
		t.Fatalf("kept %d docs, want %d", len(got), len(want))
	}
	for i, u := range want {
		if got[i].URL != u {
			t.Errorf("kept[%d] = %q, want %q", i, got[i].URL, u)
		}
	}
}

func TestCutoffEmpty(t *testing.T) {
	if got := Cutoff(nil, time.Now()); len(got) != 0 {
		t.Fatalf("Cutoff(nil) kept %d docs, want 0", len(got))
	}
}

func TestCutoffDoesNotMutateInput(t *testing.T) {
	asOf := time.Date(2021, 3, 15, 0, 0, 0, 0, time.UTC)
	docs := []Doc{
		{URL: "late", Timestamp: asOf.Add(time.Hour)},
		{URL: "ok", Timestamp: asOf.Add(-time.Hour)},
	}
	_ = Cutoff(docs, asOf)
	if docs[0].URL != "late" || docs[1].URL != "ok" {
		t.Fatal("Cutoff mutated its input slice")
	}
}
