package bench

import (
	"os"
	"path/filepath"
	"testing"
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
		wantRows       int
		wantEvidence   int
		wantLedgerRows int
	}{
		{
			name:      "found by line within tolerance",
			findings:  "| F1 | `src/render.js:9` a field with a quote is not protected | M | yes | E1 | open | me | - | - |\n",
			ledger:    "| E1 | c | cmd | i | o | m | r | observado |\n",
			key:       renderKey,
			wantFound: 1, wantBy: "line", wantRows: 1, wantEvidence: 1, wantLedgerRows: 1,
		},
		{
			name:      "found by keyword when the file is cited without a line",
			findings:  "| F1 | render.js never escapes an embedded quote | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 1, wantBy: "keyword", wantRows: 1, wantEvidence: 1,
		},
		{
			name:      "keyword alone without the file is not a match, and the row is a false positive",
			findings:  "| F1 | `src/parse.js:12` does not escape the double quote | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 1, wantRows: 1, wantEvidence: 1,
		},
		{
			name:      "line far away and no keyword is not a match",
			findings:  "| F1 | `src/render.js:40` unrelated issue | M | yes | | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 1, wantRows: 1, wantEvidence: 0,
		},
		{
			name:      "placeholder rows are not findings",
			findings:  "| (none) | | | | | | | | |\n| - | | | | | | | | |\n",
			key:       renderKey,
			wantFound: 0, wantFP: 0, wantRows: 0,
		},
		{
			name:      "a line range counts when the planted line is within tolerance of it",
			findings:  "| F1 | src/render.js:10-14 quoting | M | yes | E2 | open | me | - | - |\n",
			key:       Key{ID: "c1", Defects: []Defect{{ID: "d1", File: "src/render.js", Line: 18, Keywords: []string{"zzz"}}}},
			wantFound: 1, wantBy: "line", wantRows: 1, wantEvidence: 1,
		},
		{
			name:      "suffix paths match: fixture/src/render.js names src/render.js",
			findings:  "| F1 | fixture/src/render.js:12 broken | M | yes | E1 | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 1, wantBy: "line", wantRows: 1, wantEvidence: 1,
		},
		{
			name:      "one matching row and one unrelated row: found 1, false positive 1",
			findings:  "| F1 | src/render.js:12 escape | M | yes | E1 | open | me | - | - |\n| F2 | src/other.js:3 thing | N | yes | E2 | open | me | - | - |\n",
			key:       renderKey,
			wantFound: 1, wantBy: "line", wantFP: 1, wantRows: 2, wantEvidence: 2,
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
	r := Score(plan("| F1 | src/a.js:5 x | M | yes | E1 | open | me | - | - |\n| F2 | src/c.js beta gamma | M | yes | E1 | open | me | - | - |\n", ""), key)
	if r.Found != 2 || r.Recall != 0.5 || r.FalsePositives != 0 {
		t.Fatalf("got found %d recall %.2f fp %d", r.Found, r.Recall, r.FalsePositives)
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
	p := plan("| F1 | src/render.js:12 escape | M | yes | E1 | open | me | - | - |\n", "")
	if err := os.WriteFile(filepath.Join(ws, PlanPath), []byte(p), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := ScoreWorkspace(ws, renderKey); r.Found != 1 || r.Recall != 1 {
		t.Fatalf("got %+v", r)
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
	rows := dataRows(sectionText(doc, "Findings"))
	if len(rows) != 1 || rows[0][0] != "1" {
		t.Fatalf("rows = %v", rows)
	}
	if got := dataRows(sectionText(doc, "Missing")); got != nil {
		t.Fatalf("missing section yielded rows: %v", got)
	}
}
