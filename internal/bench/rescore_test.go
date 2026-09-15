package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The runs column was added to the aggregate after readings had already been recorded, so an older
// aggregate.json carries no runs field at all and unmarshals to the zero value. Rescore copies the field
// through rather than guessing one: a recorded 3 stays 3, and an absent count stays 0, which is the
// honest "this aggregate never recorded a run count" that no reading can be compared on. This test pins
// behavior that already held before the doc comments named it.
func TestRescoreCarriesTheRunCountItWasGiven(t *testing.T) {
	results := t.TempDir()
	// No cases: the aggregate's own fields are what this test reads, and a lookup that must never run
	// fails loudly if rescoring starts reaching for a case directory it was not given.
	noLookup := func(name string) (string, error) { return "", fmt.Errorf("unexpected case lookup: %s", name) }

	writeJSON(filepath.Join(results, "aggregate.json"), Aggregate{TS: "t0", Model: "m", Runs: 3})
	agg, err := Rescore(results, noLookup, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if agg.Runs != 3 {
		t.Fatalf("rescored runs = %d, want the 3 the source aggregate recorded", agg.Runs)
	}

	// The same measurement written before the column existed: no runs field, so the count is unknown.
	legacy := `{"ts":"t1","model":"m","cases":[],"defects":0,"found":0,"caught":0}`
	if err := os.WriteFile(filepath.Join(results, "aggregate.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	agg, err = Rescore(results, noLookup, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if agg.Runs != 0 {
		t.Fatalf("rescored runs = %d, want 0 rather than a guessed 1: the aggregate never recorded one", agg.Runs)
	}
}

// Rescore re-reads each case's kept plan and workspace with the current rules; a case whose
// workspace was removed keeps the catch it had.
func TestRescore(t *testing.T) {
	if testing.Short() {
		t.Skip("runs sh")
	}
	caseDir, key := shellCase(t)
	casesRoot := filepath.Dir(caseDir)
	results := t.TempDir()
	// Case with a kept workspace: the old aggregate said nothing was caught; the tests catch D1.
	ws := filepath.Join(results, filepath.Base(caseDir), "1", "ws")
	if err := copyTree(filepath.Join(caseDir, FixtureDir), ws); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(ws, "tests"), 0o755)
	_ = os.WriteFile(filepath.Join(ws, "tests", "d1.sh"), []byte("grep -q D1=ok src.txt\n"), 0o644)
	writePlan(t, ws, plan("| F1 | src.txt:1 x | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observado |\n"))
	_ = os.WriteFile(filepath.Join(results, filepath.Base(caseDir), "1", "test-plan.md"), []byte("stale"), 0o644)
	old := Result{Case: filepath.Base(caseDir), Run: 1, Total: 2, Found: 0, Caught: 0, CostUSD: 0.5, Turns: 7, Workspace: ws}
	// Case without a workspace: only its kept plan remains; the catch is carried over.
	gone := Result{Case: "gone", Run: 1, Total: 1, Found: 1, Caught: 1, PlanFound: true, CostUSD: 0.2, Catch: CatchResult{Checked: true, Caught: map[string]bool{"D1": true}}}
	_ = os.MkdirAll(filepath.Join(results, "gone", "1"), 0o755)
	// The gone case's retained plan declares a scoped run, so rescoring has to recalculate activation
	// from the plan alone, exactly as it recalculates the rest of the score.
	_ = os.WriteFile(filepath.Join(results, "gone", "1", "test-plan.md"), []byte("Light: a · touches cli\n\n"+plan("| F1 | src.txt:1 x | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observado |\n")), 0o644)
	writeJSON(filepath.Join(results, "aggregate.json"), Aggregate{TS: "t0", Model: "m", Cases: []Result{old, gone}, Defects: 3, Found: 1, Caught: 1, CostUSD: 0.7})
	goneDir := filepath.Join(t.TempDir(), "gone")
	_ = os.MkdirAll(filepath.Join(goneDir, FixtureDir), 0o755)
	_ = os.WriteFile(filepath.Join(goneDir, KeyFile), []byte(`{"id":"gone","language":"node","suite":"sh run-tests.sh","defects":[{"id":"D1","file":"src.txt","line":1,"keywords":["x"]}]}`), 0o644)
	_ = os.WriteFile(filepath.Join(caseDir, KeyFile), []byte(`{"id":"fake","language":"node","suite":"sh run-tests.sh","defects":[{"id":"D1","file":"src.txt","line":1,"keywords":["x"]},{"id":"D2","file":"src.txt","line":40,"keywords":["yyy"]}]}`), 0o644)
	dirs := map[string]string{filepath.Base(caseDir): caseDir, "gone": goneDir}
	lookup := func(name string) (string, error) { return dirs[name], nil }
	_ = key

	agg, err := Rescore(results, lookup, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if agg.Found != 2 || agg.Caught != 2 || agg.Defects != 3 || agg.CostUSD != 0.7 {
		t.Fatalf("aggregate: found %d caught %d defects %d cost %.2f", agg.Found, agg.Caught, agg.Defects, agg.CostUSD)
	}
	if agg.Cases[0].Caught != 1 || !agg.Cases[0].Catch.Caught["D1"] || agg.Cases[0].Turns != 7 {
		t.Fatalf("kept workspace case: %+v", agg.Cases[0])
	}
	if agg.Cases[1].Caught != 1 || agg.Cases[1].Found != 1 {
		t.Fatalf("removed workspace case: %+v", agg.Cases[1])
	}
	if !agg.Cases[1].LightActivated || agg.Cases[0].LightActivated {
		t.Fatalf("activation must come from each case's own plan: %+v", agg.Cases)
	}
	if agg.LightActivated != 1 {
		t.Fatalf("rescored activation = %d, want 1", agg.LightActivated)
	}
	data, err := os.ReadFile(filepath.Join(results, "rescored", "aggregate.json"))
	if err != nil || !strings.Contains(string(data), `"rescored_from"`) {
		t.Fatalf("rescored aggregate: %v %s", err, data)
	}
	if _, err := os.Stat(filepath.Join(results, "rescored", "summary.md")); err != nil {
		t.Fatal("no rescored summary")
	}
	_ = casesRoot
}

// A rescore whose record cannot be written must not return as if it had recorded one: the aggregate is
// written first and the summary after it, so the blocked summary is what this pins.
func TestRescoreReportsTheRecordItCouldNotWrite(t *testing.T) {
	results := t.TempDir()
	if err := writeJSON(filepath.Join(results, "aggregate.json"), Aggregate{TS: "t0", Model: "m", Runs: 1}); err != nil {
		t.Fatal(err)
	}
	// A directory where the rescored summary has to go: reading the source works, writing fails.
	if err := os.MkdirAll(filepath.Join(results, "rescored", "summary.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	noLookup := func(name string) (string, error) { return "", fmt.Errorf("unexpected case lookup: %s", name) }
	if _, err := Rescore(results, noLookup, time.Minute); err == nil {
		t.Fatal("Rescore returned as if it had recorded the rescored aggregate")
	}
}
