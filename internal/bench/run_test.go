package bench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The corpus digest must not depend on iteration order, and must change whenever the corpus
// changes: a renamed case, a renamed defect, a defect added, a defect removed.
func TestCorpusDigestIsStableAndSensitive(t *testing.T) {
	base := []CorpusCase{
		{Name: "case-a", Defects: []string{"D2", "D1"}},
		{Name: "case-b", Defects: []string{"D1"}},
	}
	reordered := []CorpusCase{
		{Name: "case-b", Defects: []string{"D1"}},
		{Name: "case-a", Defects: []string{"D1", "D2"}},
	}
	got := CorpusDigest(base)
	if want := "sha256:80f505d735845a9b"; got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
	if len(got) != len("sha256:")+16 || got[:len("sha256:")] != "sha256:" {
		t.Fatalf("digest %q is not sha256: plus 16 hex characters", got)
	}
	if CorpusDigest(reordered) != got {
		t.Fatalf("case and defect order must not change the digest: %s != %s", CorpusDigest(reordered), got)
	}
	for name, cases := range map[string][]CorpusCase{
		"changed defect id": {{Name: "case-a", Defects: []string{"D3", "D1"}}, {Name: "case-b", Defects: []string{"D1"}}},
		"changed case name": {{Name: "case-c", Defects: []string{"D2", "D1"}}, {Name: "case-b", Defects: []string{"D1"}}},
		"added defect":      {{Name: "case-a", Defects: []string{"D2", "D1"}}, {Name: "case-b", Defects: []string{"D1", "D2"}}},
		"removed defect":    {{Name: "case-a", Defects: []string{"D1"}}, {Name: "case-b", Defects: []string{"D1"}}},
	} {
		if CorpusDigest(cases) == got {
			t.Errorf("%s must change the digest", name)
		}
	}
}

// A run records the corpus it measured in the aggregate and in its history row, so a number
// whose corpus changed under it cannot read as comparable.
func TestRunRecordsCorpusInHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	benchDir := t.TempDir()
	agent := func(ctx context.Context, ws string, opts Options) (AgentResult, error) {
		return AgentResult{Result: "nothing found", CostUSD: 0.1, Turns: 3}, nil
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), BenchDir: benchDir, Agent: agent})
	want := "sha256:4cdf5fe1fd6b408e" // case-01 with its single defect d1
	if agg.Corpus != want {
		t.Fatalf("aggregate corpus = %q, want %q", agg.Corpus, want)
	}
	data, err := os.ReadFile(filepath.Join(benchDir, "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var row HistoryEntry
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	if row.Corpus != want {
		t.Fatalf("history row corpus = %q, want %q", row.Corpus, want)
	}
}
