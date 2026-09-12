package bench

import (
	"os"
	"path/filepath"
	"testing"
)

// A finding that names the defect by symbol and leaves the file citation to its evidence row is
// still a catch: the linked ledger row is part of the claim.
func TestFileCitedOnlyInTheLinkedLedgerRow(t *testing.T) {
	key := Key{ID: "n08", Defects: []Defect{{ID: "D1", File: "src/limiter.js", Line: 16, Keywords: []string{"off-by-one", "limit"}}}}
	finding := "| F1 | `allow()` lets `limit + 1` requests through | M | yes | E1 | open | me | - | - |\n"
	cases := []struct {
		name   string
		ledger string
		want   bool
	}{
		{"linked row cites the file", "| E1 | c | node -e probe against `src/limiter.js` | i | o | m | r | observado |\n", true},
		{"linked row cites another file", "| E1 | c | node -e probe against `src/other.js` | i | o | m | r | observado |\n", false},
		{"the citing row is not the linked one", "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | probe against `src/limiter.js` | i | o | m | r | observado |\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Score(plan(finding, tc.ledger), key)
			if got := r.Found == 1; got != tc.want {
				t.Fatalf("found = %v, want %v (%+v)", got, tc.want, r.Defects)
			}
		})
	}
}

func TestScoreWorkspaceReportsWhetherAPlanExisted(t *testing.T) {
	key := Key{ID: "c", Defects: []Defect{{ID: "D1", File: "src/a.js", Line: 1}}}
	ws := t.TempDir()
	if r := ScoreWorkspace(ws, key); r.PlanFound {
		t.Fatal("no plan on disk, PlanFound = true")
	}
	writePlan(t, ws, plan("", ""))
	if r := ScoreWorkspace(ws, key); !r.PlanFound {
		t.Fatal("plan on disk, PlanFound = false")
	}
}

// The plan is the run's deliverable; it survives the workspace so a later scorer can re-read it.
func TestFinishKeepsThePlanWhenItRemovesTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	writePlan(t, ws, "# plan\n")
	res := finish(Result{Workspace: ws}, Options{}, false)
	if res.Workspace != "" {
		t.Fatalf("workspace kept: %q", res.Workspace)
	}
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Fatal("workspace still on disk")
	}
	got, err := os.ReadFile(filepath.Join(dir, "test-plan.md"))
	if err != nil || string(got) != "# plan\n" {
		t.Fatalf("kept plan = %q, %v", got, err)
	}
	if r := ScorePlanFile(filepath.Join(dir, "test-plan.md"), Key{ID: "c"}); !r.PlanFound {
		t.Fatal("kept plan not scorable")
	}
}

// Activation is the run's own validated declaration, read from the plan the run left behind. A defect
// it found, a well-formed ordinary plan, or a line that merely looks like a declaration is not
// activation — this is the signal that separates "the mode ran" from "the mode never ran".
func TestScoringReadsActivationFromThePlan(t *testing.T) {
	key := Key{ID: "c", Defects: []Defect{{ID: "D1", File: "src/a.js", Line: 1, Keywords: []string{"clamp"}}}}
	finding := "| F1 | `src/a.js:1` clamps to the maximum | M | yes | E1 | open | me | - | - |\n"
	ledger := "| E1 | c | node | i | o | m | r | observado |\n"
	scoped := func(declaration string) string { return declaration + plan(finding, ledger) }
	cases := []struct {
		name string
		doc  string
		want bool
	}{
		{"a validated declaration activates", scoped("Light: a · touches cli\n\n"), true},
		{"no declaration never activates", scoped(""), false},
		{"a declaration without the shape does not activate", scoped("Light: a\n\n"), false},
		{"a target the plan never ranked does not activate", scoped("Light: zzz · touches cli\n\n"), false},
		{"a skipped layer with no reason does not activate", scoped("Light: a · touches cli\n\n") +
			"\n## Layer matrix\n\n| Layer | Skill | Scope | Status |\n|---|---|---|---|\n| Persistence | `x` | | n/a |\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Score(tc.doc, key).LightActivated; got != tc.want {
				t.Fatalf("LightActivated = %v, want %v", got, tc.want)
			}
		})
	}
	// The run that finds the defect by writing an ordinary plan is exactly what this signal has to keep
	// apart from a scoped run: detection is not activation.
	r := Score(scoped(""), key)
	if r.Found != 1 {
		t.Fatalf("an ordinary plan must still find the defect: %+v", r)
	}
	if r.LightActivated {
		t.Fatal("finding a defect is not activation")
	}
	// And the signal survives the path the bench actually reads.
	ws := t.TempDir()
	writePlan(t, ws, scoped("Light: a · touches cli\n\n"))
	if r := ScoreWorkspace(ws, key); !r.PlanFound || !r.LightActivated {
		t.Fatalf("ScoreWorkspace dropped activation: %+v", r)
	}
}

func writePlan(t *testing.T, ws, content string) {
	t.Helper()
	p := filepath.Join(ws, PlanPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Keywords are matched in the finding row only: a linked ledger row may name the file, but its
// prose must not credit a defect the finding does not describe ("attempts" in a retry ledger).
func TestKeywordsInTheLinkedLedgerRowDoNotCount(t *testing.T) {
	key := Key{ID: "n06", Defects: []Defect{
		{ID: "D1", File: "src/retry.js", Line: 11, Keywords: []string{"429", "attempts"}},
		{ID: "D2", File: "src/retry.js", Line: 18, Keywords: []string{"fail-open", "null"}},
	}}
	finding := "| F1 | `src/retry.js:18` returns ok with null data when retries are exhausted | M | yes | E1 | open | me | - | - |\n"
	ledger := "| E1 | c | probe with maxAttempts 3 against src/retry.js | i | o | m | r | observado |\n"
	r := Score(plan(finding, ledger), key)
	if r.Found != 1 || !r.Defects[1].Found || r.Defects[0].Found {
		t.Fatalf("found = %d, defects = %+v", r.Found, r.Defects)
	}
}
