package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// fakeCaseNamed writes the same shape of case as fakeCase under a chosen name: the ordering and
// concurrency assertions need cases whose names differ.
func fakeCaseNamed(t *testing.T, root, name string) string {
	t.Helper()
	caseDir := filepath.Join(root, name)
	fx := filepath.Join(caseDir, FixtureDir)
	for p, body := range map[string]string{
		"package.json":     `{"name":"fx","type":"module","scripts":{"test":"node --test"}}`,
		"src/a.mjs":        "export const add = (a, b) => a + b;\n",
		"tests/a.test.mjs": "import { test } from 'node:test';\nimport assert from 'node:assert/strict';\nimport { add } from '../src/a.mjs';\ntest('adds', () => assert.equal(add(1, 2), 3));\n",
	} {
		full := filepath.Join(fx, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	key := fmt.Sprintf(`{"id":%q,"language":"node","suite":"node --test","surface":"lib","defects":[{"id":"d1","file":"src/a.mjs","line":1,"class":"arith","keywords":["overflow"],"description":"x","trigger":{"input":"i","expected":"e","actual":"a"},"why_missed":"w"}]}`, name)
	if err := os.WriteFile(filepath.Join(caseDir, KeyFile), []byte(key), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseDir
}

// A run told to use workers must reach the pool: a sequential loop produces the same aggregate as
// a concurrent one, so nothing else in this file can see whether the flag does anything.
func TestRunRunsCasesSideBySideWithWorkers(t *testing.T) {
	requireNodeAndGit(t)
	root := t.TempDir()
	var globs []string
	for _, name := range []string{"case-a", "case-b", "case-c"} {
		globs = append(globs, fakeCaseNamed(t, root, name))
	}
	var mu sync.Mutex
	live, peak := 0, 0
	agent := func(ctx context.Context, ws string, opts Options) (AgentResult, error) {
		mu.Lock()
		live++
		if live > peak {
			peak = live
		}
		mu.Unlock()
		// Hold the slot so two cases can be inside at once. However slow this is, a sequential run
		// can never report a peak above one.
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		live--
		mu.Unlock()
		return AgentResult{Result: "nothing found", CostUSD: 0.01, Turns: 2}, nil
	}
	agg, code := Run(Options{CasesGlob: strings.Join(globs, ","), Runs: 1, Workers: 3, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), BenchDir: t.TempDir(), Agent: agent})
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if len(agg.Cases) != 3 {
		t.Fatalf("%d cases in the aggregate, want 3", len(agg.Cases))
	}
	if peak < 2 {
		t.Fatalf("Workers 3 ran at most %d case(s) at once: --concurrency never reaches the pool", peak)
	}
	if peak > 3 {
		t.Fatalf("%d cases ran at once, want at most 3", peak)
	}
}

// Whatever finishes first, the aggregate must stay in case order: a report that reorders itself
// when the machine is busy is not the same report.
func TestRunKeepsCaseOrderWhateverFinishesFirst(t *testing.T) {
	requireNodeAndGit(t)
	root := t.TempDir()
	names := []string{"case-a", "case-b", "case-c"}
	var globs []string
	for _, name := range names {
		globs = append(globs, fakeCaseNamed(t, root, name))
	}
	agent := func(ctx context.Context, ws string, opts Options) (AgentResult, error) {
		for i, name := range names {
			if strings.Contains(ws, name) {
				// The first case is the slowest, so completion order is the reverse of case order.
				time.Sleep(time.Duration(len(names)-i) * 150 * time.Millisecond)
			}
		}
		return AgentResult{Result: "nothing found", CostUSD: 0.01, Turns: 2}, nil
	}
	agg, code := Run(Options{CasesGlob: strings.Join(globs, ","), Runs: 2, Workers: 3, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), BenchDir: t.TempDir(), Agent: agent})
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if want := len(names) * 2; len(agg.Cases) != want {
		t.Fatalf("%d cases in the aggregate, want %d", len(agg.Cases), want)
	}
	i := 0
	for _, name := range names {
		for run := 1; run <= 2; run++ {
			if got := agg.Cases[i]; got.Case != name || got.Run != run {
				t.Fatalf("aggregate[%d] = %s #%d, want %s #%d: the order followed completion", i, got.Case, got.Run, name, run)
			}
			i++
		}
	}
}

// The ceiling stops launching work once it is crossed while the units already in flight finish,
// which is why the total can land just past the ceiling.
func TestRunCostCeilingStopsLaunchingMoreCases(t *testing.T) {
	requireNodeAndGit(t)
	root := t.TempDir()
	var globs []string
	for _, name := range []string{"case-a", "case-b", "case-c", "case-d"} {
		globs = append(globs, fakeCaseNamed(t, root, name))
	}
	agent := func(ctx context.Context, ws string, opts Options) (AgentResult, error) {
		time.Sleep(150 * time.Millisecond)
		return AgentResult{Result: "nothing found", CostUSD: 1.00, Turns: 2}, nil
	}
	agg, code := Run(Options{CasesGlob: strings.Join(globs, ","), Runs: 1, Workers: 2, MaxCostUSD: 1.00, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), BenchDir: t.TempDir(), Agent: agent})
	if code != ExitCostCeiling {
		t.Fatalf("exit code %d, want %d", code, ExitCostCeiling)
	}
	if !agg.CostCeilingHit {
		t.Fatal("the aggregate must record that the ceiling was hit")
	}
	if len(agg.Cases) != 2 {
		t.Fatalf("%d cases ran, want the 2 that were already in flight", len(agg.Cases))
	}
	if agg.CostUSD < 1.00 {
		t.Fatalf("cost %v, want the in-flight units counted past the ceiling", agg.CostUSD)
	}
}

// syncBuffer is a log sink safe to read while the run is still writing to it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// A run must say what finished while it is still running: the aggregate is ordered by case, but the
// log is progress, and silence until the end hides a corpus that stalled on its first case.
func TestRunLogsEachCaseAsItFinishes(t *testing.T) {
	requireNodeAndGit(t)
	root := t.TempDir()
	var globs []string
	for _, name := range []string{"case-a", "case-b", "case-c"} {
		globs = append(globs, fakeCaseNamed(t, root, name))
	}
	release := make(chan struct{})
	agent := func(ctx context.Context, ws string, opts Options) (AgentResult, error) {
		if strings.Contains(ws, "case-c") {
			// Hold the last case open until the test has seen the other two lines.
			<-release
		}
		return AgentResult{Result: "nothing found", CostUSD: 0.01, Turns: 2}, nil
	}
	log := &syncBuffer{}
	done := make(chan Aggregate, 1)
	go func() {
		agg, _ := Run(Options{CasesGlob: strings.Join(globs, ","), Runs: 1, Workers: 3, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), BenchDir: t.TempDir(), Agent: agent, Log: log})
		done <- agg
	}()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if seen := log.String(); strings.Contains(seen, "case-a") && strings.Contains(seen, "case-b") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	seen := log.String()
	close(release)
	agg := <-done
	for _, name := range []string{"case-a", "case-b"} {
		if !strings.Contains(seen, name) {
			t.Fatalf("the log said nothing about %s while it had already finished; it was:\n%s", name, seen)
		}
	}
	if len(agg.Cases) != 3 {
		t.Fatalf("%d cases in the aggregate, want 3", len(agg.Cases))
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
