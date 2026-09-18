package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plancheck "github.com/alesierraalta/rdd-plus/internal/plan"
)

const findingsHeader = "| Id | Finding (path:line, one line) | Severity | Data safe? | Evidence id | Status | Verdict | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|\n"
const ledgerHeader = "| Id | Claim | Executed | Inputs | Observed | Mutation | Reproduction | Label |\n|---|---|---|---|---|---|---|---|\n"

func plan(findings, ledger string) string {
	return "# Test plan\n\n## Ranked targets\n\n| Target | Status |\n|---|---|\n| a | done |\n\n" +
		"## Findings\n\nA rejected finding is a known non-issue.\n\n" + findingsHeader + findings +
		"\nStatuses: open · fixed.\n\n## Evidence ledger\n\n" + ledgerHeader + ledger + "\n### Hypotheses (razonado)\n\n| Id | H |\n|---|---|\n"
}

var renderKey = Key{ID: "c1", Language: "node", Suite: "node --test", Defects: []Defect{
	{ID: "d1", File: "src/render.js", Line: 12, Keywords: []string{"escape", "double quote"}},
}}

func TestScore(t *testing.T) {
	cases := []struct {
		name           string
		findings       string
		ledger         string
		key            Key
		wantFound      int
		wantBy         string
		wantFP         int
		wantPending    int
		wantRows       int
		wantEvidence   int
		wantLedgerRows int
	}{
		{
			name:      "found by line within tolerance",
			findings:  "| F1 | `src/render.js:9` a field with a quote is not protected | M | yes | E1 | open | me | - | - |\n",
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n",
			key:       renderKey,
			wantFound: 1, wantBy: "line", wantPending: 1, wantRows: 1, wantEvidence: 1, wantLedgerRows: 1,
		},
		{
			name:      "found by keyword when the file is cited without a line",
			findings:  "| F1 | render.js never escapes an embedded quote | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n",
			wantFound: 1, wantBy: "keyword", wantPending: 1, wantRows: 1, wantEvidence: 1, wantLedgerRows: 2,
		},
		{
			// An unmatched row only proposes no association; without adjudication it remains pending, not a false positive.
			name:      "keyword alone without the file is not a proposed match",
			findings:  "| F1 | `src/parse.js:12` does not escape the double quote | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 0, wantPending: 1, wantRows: 1, wantEvidence: 1,
		},
		{
			// An unmatched row only proposes no association; without adjudication it remains pending, not a false positive.
			name:      "line far away and no keyword is not a proposed match",
			findings:  "| F1 | `src/render.js:40` unrelated issue | M | yes | | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 0, wantPending: 1, wantRows: 1, wantEvidence: 0,
		},
		{
			name:      "placeholder rows are not findings",
			findings:  "| (none) | | | | | | | | |\n| - | | | | | | | | |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 0, wantPending: 0, wantRows: 0,
		},
		{
			name:      "a line range counts when the planted line is within tolerance of it",
			findings:  "| F1 | src/render.js:10-14 quoting | M | yes | E2 | open | me | - | - |\n",
			key:       Key{ID: "c1", Defects: []Defect{{ID: "d1", File: "src/render.js", Line: 18, Keywords: []string{"zzz"}}}},
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n",
			wantFound: 1, wantBy: "line", wantPending: 1, wantRows: 1, wantEvidence: 1, wantLedgerRows: 2,
		},
		{
			name:      "suffix paths match: fixture/src/render.js names src/render.js",
			findings:  "| F1 | fixture/src/render.js:12 broken | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n",
			wantFound: 1, wantBy: "line", wantPending: 1, wantRows: 1, wantEvidence: 1, wantLedgerRows: 2,
		},
		{
			// The unrelated row is pending until a human or verified rule adjudicates it.
			name:      "one matching row and one unrelated row: found 1, pending 1",
			findings:  "| F1 | src/render.js:12 escape | M | yes | E1 | open | me | - | - |\n| F2 | src/other.js:3 thing | N | yes | E2 | open | me | - | - |\n",
			key:       renderKey,
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n",
			wantFound: 1, wantBy: "line", wantFP: 0, wantPending: 2, wantRows: 2, wantEvidence: 2, wantLedgerRows: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Score(plan(tc.findings, tc.ledger), tc.key)
			if r.Found != tc.wantFound {
				t.Fatalf("found = %d, want %d (%+v)", r.Found, tc.wantFound, r.Defects)
			}
			if tc.wantFound > 0 && r.Defects[0].MatchedBy != tc.wantBy {
				t.Fatalf("matched_by = %q, want %q", r.Defects[0].MatchedBy, tc.wantBy)
			}
			if r.FalsePositives != tc.wantFP {
				t.Fatalf("false positives = %d, want %d", r.FalsePositives, tc.wantFP)
			}
			if r.PendingAdjudication != tc.wantPending {
				t.Fatalf("pending adjudication = %d, want %d", r.PendingAdjudication, tc.wantPending)
			}
			if r.FindingRows != tc.wantRows {
				t.Fatalf("finding rows = %d, want %d", r.FindingRows, tc.wantRows)
			}
			if r.FindingsWithEvidence != tc.wantEvidence {
				t.Fatalf("findings with evidence = %d, want %d", r.FindingsWithEvidence, tc.wantEvidence)
			}
			if r.LedgerRows != tc.wantLedgerRows {
				t.Fatalf("ledger rows = %d, want %d", r.LedgerRows, tc.wantLedgerRows)
			}
			if r.Total != len(tc.key.Defects) {
				t.Fatalf("total = %d", r.Total)
			}
		})
	}
}

func TestScoreRecallOverSeveralDefects(t *testing.T) {
	key := Key{ID: "c", Defects: []Defect{
		{ID: "a", File: "src/a.js", Line: 5, Keywords: []string{"alpha"}},
		{ID: "b", File: "src/b.js", Line: 5, Keywords: []string{"beta"}},
		{ID: "c", File: "src/c.js", Line: 5, Keywords: []string{"gamma"}},
		{ID: "d", File: "src/d.js", Line: 5, Keywords: []string{"delta"}},
	}}
	r := Score(plan("| F1 | src/a.js:5 x | M | yes | E1 | open | me | - | - |\n| F2 | src/c.js beta gamma | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n"), key)
	if r.Found != 2 || r.Recall != 0.5 || r.FalsePositives != 0 {
		t.Fatalf("got found %d recall %.2f fp %d", r.Found, r.Recall, r.FalsePositives)
	}
}

func TestScoreCleanControlKeepsFindingsPending(t *testing.T) {
	key := Key{ID: "clean", Control: ControlClean}
	r := Score(plan("| F1 | src/allocate.js:1 invented issue | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observed |\n"), key)
	if r.Total != 0 || !r.Control || r.Recall != 0 || r.FalsePositives != 0 {
		t.Fatalf("clean score = total %d control %v recall %.2f false positives %d", r.Total, r.Control, r.Recall, r.FalsePositives)
	}
	if r.UnmatchedFindings != 1 || r.PendingAdjudication != 1 {
		t.Fatalf("clean finding = unmatched %d pending %d, want one of each", r.UnmatchedFindings, r.PendingAdjudication)
	}
	if r.AdjudicationComplete {
		t.Fatal("clean finding was marked adjudicated")
	}

	empty := Score(plan("", ""), key)
	if !empty.AdjudicationComplete {
		t.Fatal("clean control with no findings is not adjudication-complete")
	}
	if empty.Precision != nil {
		t.Fatalf("precision = %v, want null", *empty.Precision)
	}
}

func TestScoreWorkspaceWithoutPlan(t *testing.T) {
	r := ScoreWorkspace(t.TempDir(), renderKey)
	if r.Recall != 0 || r.Found != 0 || len(r.Notes) != 1 || r.Notes[0] != "no plan" {
		t.Fatalf("got %+v", r)
	}
}

func TestScoreWorkspaceReadsThePlanFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "docs", "testing"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := plan("| F1 | src/render.js:12 escape | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observado |\n| E2 | c | cmd | i | o | m | r | observado |\n")
	if err := os.WriteFile(filepath.Join(ws, PlanPath), []byte(p), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := ScoreWorkspace(ws, renderKey); r.Found != 1 || r.Recall != 1 {
		t.Fatalf("got %+v", r)
	}
}

// A run that delivers its plan at the path its .rdd-plus.json declares is scored from that path; a
// declaration that escapes is refused with a note and the default path is read instead; and a
// workspace with no plan anywhere keeps today's result and note.
// A declared path that exists but cannot be read as a plan is not a missing plan, and the run's record must
// not say it is. The two scorers differ today: the plain one treats the read failure as an absent plan and
// says the path exists instead, while the adjudicated one refuses and hands the error back. Pinning both
// keeps the difference visible rather than letting either claim the path was not found.
func TestScoreWorkspaceOnADeclaredPathThatCannotBeReadAsAPlan(t *testing.T) {
	ws := t.TempDir()
	declarePlan(t, ws, "docs/testing/plan-dir")
	if err := os.MkdirAll(filepath.Join(ws, "docs/testing/plan-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := ScoreWorkspace(ws, renderKey)
	if r.PlanFound || r.PlanPath != "" {
		t.Fatalf("PlanFound = %v, PlanPath = %q; an unread plan credits nothing", r.PlanFound, r.PlanPath)
	}
	want := `declared plan "docs/testing/plan-dir" exists but was not read as a plan`
	if fmt.Sprint(r.Notes) != fmt.Sprint([]string{"no plan", want}) {
		t.Fatalf("notes = %v, want exactly [no plan, %s]", r.Notes, want)
	}
	if _, err := ScoreWorkspaceWithAdjudication(ws, renderKey, nil); err == nil {
		t.Fatalf("the adjudicated scorer no longer refuses an unreadable declared plan")
	}
}

func TestScoreWorkspaceResolvesTheDeclaredPlanPath(t *testing.T) {
	body := plan("| F1 | src/render.js:12 escape | M | yes | E1 | open | me | - | - |\n", "| E1 | c | cmd | i | o | m | r | observado |\n")
	cases := []struct {
		name           string
		setup          func(t *testing.T, ws string)
		wantFound      bool
		wantPath       string // PlanPath when a plan was found; "" means it must stay empty
		wantNotes      []string
		wantExactNotes []string
		wantNoteCount  int
		wantNoNotes    bool
	}{
		{
			name: "declared elsewhere",
			setup: func(t *testing.T, ws string) {
				declarePlan(t, ws, "docs/testing/test-plan-other.md")
				writePlanAt(t, ws, "docs/testing/test-plan-other.md", body)
			},
			wantFound: true, wantPath: "docs/testing/test-plan-other.md", wantNoNotes: true,
		},
		{
			name: "declared plan absent, default plan present",
			setup: func(t *testing.T, ws string) {
				declarePlan(t, ws, "docs/testing/test-plan-other.md")
				writePlan(t, ws, body)
			},
			wantFound: false,
			wantExactNotes: []string{
				"no plan",
				`declared plan "docs/testing/test-plan-other.md" was not found; default plan "docs/testing/test-plan.md" was not read`,
			},
		},
		{
			name: "declared plan absent, default plan absent",
			setup: func(t *testing.T, ws string) {
				declarePlan(t, ws, "docs/testing/test-plan-other.md")
			},
			wantFound: false,
			wantExactNotes: []string{
				"no plan",
				`declared plan "docs/testing/test-plan-other.md" was not found; default plan "docs/testing/test-plan.md" holds nothing either`,
			},
		},
		{
			name:           "no plan anywhere",
			setup:          func(t *testing.T, ws string) {},
			wantFound:      false,
			wantExactNotes: []string{"no plan"},
		},
		{
			name: "a \"../\" declaration is refused",
			setup: func(t *testing.T, ws string) {
				declarePlan(t, ws, "../escape.md")
				writePlan(t, ws, body)
			},
			wantFound: true, wantPath: PlanPath, wantNoteCount: 1,
			wantNotes: []string{"declared plan path refused", `escapes the worktree: "../escape.md"`},
		},
		{
			name: "an absolute declaration is refused",
			setup: func(t *testing.T, ws string) {
				declarePlan(t, ws, filepath.Join(ws, "outside.md"))
				writePlan(t, ws, body)
			},
			wantFound: true, wantPath: PlanPath, wantNoteCount: 1,
			wantNotes: []string{"declared plan path refused", "must be repository-relative", "is absolute"},
		},
	}
	scorers := []struct {
		name  string
		score func(t *testing.T, ws string, key Key) Result
	}{
		{"ScoreWorkspace", func(_ *testing.T, ws string, key Key) Result { return ScoreWorkspace(ws, key) }},
		{"ScoreWorkspaceWithAdjudication", func(t *testing.T, ws string, key Key) Result {
			r, err := ScoreWorkspaceWithAdjudication(ws, key, nil)
			if err != nil {
				t.Fatalf("ScoreWorkspaceWithAdjudication: %v", err)
			}
			return r
		}},
	}
	for _, sc := range scorers {
		for _, tc := range cases {
			t.Run(sc.name+"/"+tc.name, func(t *testing.T) {
				ws := t.TempDir()
				tc.setup(t, ws)
				r := sc.score(t, ws, renderKey)
				if r.PlanFound != tc.wantFound {
					t.Fatalf("PlanFound = %v, want %v (%+v)", r.PlanFound, tc.wantFound, r)
				}
				if r.PlanPath != tc.wantPath {
					t.Fatalf("PlanPath = %q, want %q", r.PlanPath, tc.wantPath)
				}
				if tc.wantFound && r.Found == 0 {
					t.Fatalf("the plan that was read credited no defect: %+v", r)
				}
				if tc.wantNoNotes && len(r.Notes) != 0 {
					t.Fatalf("notes = %v, want none: a plan was read and nothing was overridden", r.Notes)
				}
				joined := strings.Join(r.Notes, "\n")
				if tc.wantNoteCount > 0 && len(r.Notes) != tc.wantNoteCount {
					t.Fatalf("notes = %v, want %d note(s)", r.Notes, tc.wantNoteCount)
				}
				for _, want := range tc.wantNotes {
					if !strings.Contains(joined, want) {
						t.Fatalf("notes %v do not contain %q", r.Notes, want)
					}
				}
				if tc.wantExactNotes != nil && fmt.Sprint(r.Notes) != fmt.Sprint(tc.wantExactNotes) {
					t.Fatalf("notes = %v, want %v", r.Notes, tc.wantExactNotes)
				}
				if !tc.wantFound && !strings.Contains(joined, "no plan") {
					t.Fatalf("notes %v lack the no-plan note", r.Notes)
				}
				if tc.wantFound && strings.Contains(joined, "no plan") {
					t.Fatalf("a plan was read and still carries a no-plan note: %v", r.Notes)
				}
			})
		}
	}
}

// declarePlan writes the workspace's plan declaration.
func declarePlan(t *testing.T, ws, declared string) {
	t.Helper()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"planPath":%q}`, declared)
	if err := os.WriteFile(filepath.Join(ws, plancheck.ConfigName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writePlanAt writes a plan at a workspace-relative path.
func writePlanAt(t *testing.T, ws, rel, content string) {
	t.Helper()
	p := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSamePath(t *testing.T) {
	cases := []struct {
		cited, key string
		want       bool
	}{
		{"src/render.js", "src/render.js", true},
		{"render.js", "src/render.js", true},
		{"fixture/src/render.js", "src/render.js", true},
		{"./src/render.js", "src/render.js", true},
		{"src/parse.js", "src/render.js", false},
		{"xrender.js", "src/render.js", false},
		{"src/render.jsx", "src/render.js", false},
	}
	for _, tc := range cases {
		if got := samePath(tc.cited, tc.key); got != tc.want {
			t.Errorf("samePath(%q, %q) = %v, want %v", tc.cited, tc.key, got, tc.want)
		}
	}
}

func TestSectionAndRows(t *testing.T) {
	doc := "## Findings\n\ntext\n\n| a | b |\n|---|---|\n| 1 | x |\n| (none) | |\n\nafter\n\n## Next\n\n| z |\n|---|\n| 9 |\n"
	_, rows := plancheck.Table(doc, "Findings")
	if len(rows) != 1 || rows[0][0] != "1" {
		t.Fatalf("rows = %v", rows)
	}
	if _, got := plancheck.Table(doc, "Missing"); got != nil {
		t.Fatalf("missing section yielded rows: %v", got)
	}
}

// A blank line inside a table is not the end of it. The plan's own reader skips one (`table` says so in as
// many words) and this scanner stopped there instead, so a finding or an evidence row under a blank line was
// written by the run, reported by the plan, and invisible to the score.
func TestScoreReadsTheRowsUnderABlankLineInsideATable(t *testing.T) {
	key := Key{ID: "c", Defects: []Defect{
		{ID: "a", File: "src/a.js", Line: 5, Keywords: []string{"alpha"}},
		{ID: "b", File: "src/b.js", Line: 5, Keywords: []string{"beta"}},
	}}
	findings := "| F1 | `src/a.js:5` x | M | yes | E1 | open | me | - | - |\n" +
		"\n" +
		"| F2 | `src/b.js:5` beta | M | yes | E2 | open | me | - | - |\n"
	ledger := "| E1 | c | cmd | i | o | m | r | observado |\n" +
		"\n" +
		"| E2 | c | cmd | i | o | m | r | observado |\n"
	r := Score(plan(findings, ledger), key)
	if r.FindingRows != 2 || r.LedgerRows != 2 {
		t.Fatalf("rows = %d findings, %d ledger: a blank line inside the table hid the rows under it", r.FindingRows, r.LedgerRows)
	}
	if r.Found != 2 {
		t.Fatalf("found = %d of 2: the row under the blank line was never scored", r.Found)
	}
}

func TestScoreReadsTheRealFindingsTableAfterAFencedExample(t *testing.T) {
	key := Key{ID: "f43", Defects: []Defect{{
		ID: "D1", File: "src/real.js", Line: 12, Keywords: []string{"real"},
	}}}
	findings := "| F1 | src/real.js:12 real finding | M | yes | E1 | open | me | - | - |\n"
	ledger := "| E1 | claim | cmd | inputs | observed | mutation | reproduction | observado |\n"
	doc := plan(findings, ledger)
	example := "```markdown\n| Example | Value |\n|---|---|\n| F9 | src/fenced.js:1 documentation |\n```\n\n"
	doc = strings.Replace(doc, findingsHeader, example+findingsHeader, 1)

	r := Score(doc, key)
	if r.Found != 1 || r.FindingRows != 1 || r.Defects[0].ID != "D1" {
		t.Fatalf("score = %+v, want the real findings row after the fenced example", r)
	}
}
