package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendHistoryIsAdditive(t *testing.T) {
	dir := t.TempDir()
	for i, r := range []float64{0.5, 0.8} {
		if err := AppendHistory(dir, HistoryEntry{TS: "t" + string(rune('0'+i)), Out: "o", Model: "m", Cases: 1, Defects: 4, Found: 2, Recall: r, SkillVersion: "3.0"}); err != nil {
			t.Fatal(err)
		}
	}
	jsonl, _ := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	if n := strings.Count(strings.TrimSpace(string(jsonl)), "\n") + 1; n != 2 {
		t.Fatalf("jsonl lines = %d, want 2: %s", n, jsonl)
	}
	md, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if strings.Count(string(md), "# Benchmark history") != 1 {
		t.Fatalf("header must be written once: %s", md)
	}
	if strings.Count(string(md), "| t0 |") != 1 || strings.Count(string(md), "| t1 |") != 1 {
		t.Fatalf("both rows expected once: %s", md)
	}
	if !strings.Contains(string(md), "| 0.80 |") {
		t.Fatalf("recall formatting: %s", md)
	}
}

func TestSkillVersion(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(f, []byte("---\nname: test-strategy\nmetadata:\n  author: x\n  version: \"3.0\"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SkillVersion(f); got != "3.0" {
		t.Fatalf("version = %q", got)
	}
	if got := SkillVersion(filepath.Join(dir, "missing.md")); got != "unknown" {
		t.Fatalf("missing file version = %q", got)
	}
	if err := os.WriteFile(f, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SkillVersion(f); got != "unknown" {
		t.Fatalf("no version field = %q", got)
	}
}

// A rescore re-reads an earlier run: it spends nothing and belongs to that run's skill version.
// Its row must say so, or a reader summing the cost column counts the same money twice.
func TestRescoreRowIsMarkedAndCostsNothing(t *testing.T) {
	dir := t.TempDir()
	run := HistoryEntry{TS: "2026-01-01T00:00:00Z", Out: "r1", Model: "m", Cases: 2, Defects: 4, Found: 2, Recall: 0.5, Caught: 1, CostUSD: 3.5, SkillVersion: "3.0"}
	if err := AppendHistory(dir, run); err != nil {
		t.Fatal(err)
	}
	rescore := HistoryEntry{TS: "2026-01-02T00:00:00Z", Out: "r1/rescored", Model: "m", Cases: 2, Defects: 4, Found: 2, Recall: 0.5, Caught: 3, RecallCaught: 0.75, CostUSD: 3.5, SkillVersion: "3.0", Kind: KindRescore, RunTS: run.TS, SourceRun: "r1"}
	if err := AppendHistory(dir, rescore); err != nil {
		t.Fatal(err)
	}
	jsonl, _ := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	var rows []HistoryEntry
	for _, line := range strings.Split(strings.TrimSpace(string(jsonl)), "\n") {
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, e)
	}
	if rows[0].Kind != KindRun {
		t.Fatalf("a run row must say so: %+v", rows[0])
	}
	if rows[1].Kind != KindRescore || rows[1].SourceRun != "r1" || rows[1].RunTS != run.TS {
		t.Fatalf("rescore row: %+v", rows[1])
	}
	if rows[1].CostUSD != 0 {
		t.Fatalf("a rescore spends nothing, got $%.2f", rows[1].CostUSD)
	}
	if rows[1].SkillVersion != "3.0" {
		t.Fatalf("a rescore keeps the skill version of the run it re-reads: %q", rows[1].SkillVersion)
	}
	md, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if !strings.Contains(string(md), "| rescore of r1 |") {
		t.Fatalf("markdown must name the source run: %s", md)
	}
	if !strings.Contains(string(md), "reported and caught are independent") {
		t.Fatalf("the header must say caught can exceed reported: %s", md)
	}
}

// Two rescores of one run can disagree when the scoring rules change between them. A row that
// does not say which build produced it turns that into a contradiction nobody can resolve.
func TestEveryRowRecordsTheScorerThatProducedIt(t *testing.T) {
	dir := t.TempDir()
	run := HistoryEntry{TS: "t0", Out: "r1", Model: "m", Cases: 1, Defects: 4, Found: 2, Caught: 2, SkillVersion: "3.1"}
	if err := AppendHistory(dir, run); err != nil {
		t.Fatal(err)
	}
	rescore := HistoryEntry{TS: "t1", Out: "r1/rescored", Model: "m", Cases: 1, Defects: 4, Found: 2, Caught: 3, SkillVersion: "3.1", Kind: KindRescore, RunTS: "t0", SourceRun: "r1"}
	if err := AppendHistory(dir, rescore); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		if e.Scorer == "" {
			t.Fatalf("row %d records no scorer: %s", i+1, line)
		}
	}
	md, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if !strings.Contains(string(md), "| scorer |") {
		t.Fatalf("the markdown table must carry the scorer column: %s", md)
	}
	if !strings.Contains(string(md), "supersedes") {
		t.Fatalf("the intro must say a later rescore supersedes an earlier one: %s", md)
	}
}

// An explicit scorer is kept as given, so a row can be attributed to the build it came from.
func TestScorerCanBeStated(t *testing.T) {
	dir := t.TempDir()
	if err := AppendHistory(dir, HistoryEntry{TS: "t", Out: "o", Model: "m", Scorer: "abc1234"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "history.jsonl"))
	if !strings.Contains(string(data), `"scorer":"abc1234"`) {
		t.Fatalf("scorer not kept: %s", data)
	}
}

func TestScorerRevisionIsNeverEmpty(t *testing.T) {
	if scorerRevision() == "" {
		t.Fatal("a row must always be attributable to something, even outside a repository")
	}
}

// A build made from an uncommitted tree cannot be recovered from its commit, so numbers recorded
// under it are not reproducible. The row already says so; the run has to say it out loud, before
// the money is spent rather than after.
func TestDirtyScorerIsAnnounced(t *testing.T) {
	cases := map[string]bool{"abc1234": false, "abc1234+dirty": true, "unknown": true}
	for rev, want := range cases {
		if got := ScorerIsProvisional(rev); got != want {
			t.Errorf("ScorerIsProvisional(%q) = %v, want %v", rev, got, want)
		}
	}
	msg := ProvisionalScorerWarning("abc1234+dirty")
	for _, want := range []string{"abc1234+dirty", "not reproducible", "commit"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("warning missing %q: %s", want, msg)
		}
	}
}
