package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Init writes the shipped skeleton so the flow fills tables instead of inventing a structure.
func TestInitWritesTheTemplateAndRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docs", "testing", "test-plan.md")
	if err := Init(p, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Findings", "## Evidence ledger", "| Id | Finding"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("template missing %q", want)
		}
	}
	if err := Init(p, false); err == nil {
		t.Fatal("an existing plan must not be silently overwritten")
	}
	write(t, dir, "docs/testing/test-plan.md", "mine")
	if err := Init(p, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) == "mine" {
		t.Fatal("--force must rewrite the file")
	}
}

func TestCheckAcceptsACompliantPlan(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plan.md")
	if err := Init(p, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(p)
	plan := strings.Replace(string(body),
		"| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Pinning test (suite path :: test name) | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |\n|---|---|---|---|---|---|---|---|---|---|\n",
		"| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Pinning test (suite path :: test name) | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |\n|---|---|---|---|---|---|---|---|---|---|\n"+
			"| F1 | `src/a.js:5` drops a quoted comma | data loss | yes | E1 | tests/a.test.js :: keeps a quoted comma | fixed | me / 2026-09-10 | - | abc123 |\n", 1)
	plan = strings.Replace(plan,
		"| Id | Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n|---|---|---|---|---|---|---|---|\n",
		"| Id | Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n|---|---|---|---|---|---|---|---|\n"+
			"| E1 | it drops the comma | `node --test` | `a,\"b,c\"` | 3 fields | reverted → red | same input | observado |\n", 1)
	if err := os.WriteFile(p, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("compliant plan rejected: %v", problems)
	}
}

func TestCheckNamesEveryContractBreach(t *testing.T) {
	cases := []struct {
		name    string
		plan    string
		wantSub string
	}{
		{
			name:    "findings written as prose",
			plan:    "## Findings\n\n### F1 — something broke\n- Severity: high\n\n## Evidence ledger\n\n| Id | Claim |\n|---|---|\n",
			wantSub: "not a table",
		},
		{
			name: "a finding that cites no path",
			plan: header + "| F1 | `formatCents` rounds a tie down | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n",
			wantSub: "cites no path:line",
		},
		{
			name: "a finding citing an evidence id that does not exist",
			plan: header + "| F1 | `src/a.js:5` x | M | yes | E9 | t.js :: x | fixed | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n",
			wantSub: "cites evidence E9",
		},
		{
			name: "a confirmed finding with no pinning test",
			plan: header + "| F1 | `src/a.js:5` x | M | yes | E1 |  | fixed | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n",
			wantSub: "names no pinning test",
		},
		{
			name: "a razonado row inside the evidence ledger",
			plan: header + "| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | razonado |\n",
			wantSub: "labelled razonado",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := write(t, dir, "plan.md", tc.plan)
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tc.wantSub) {
				t.Fatalf("problems = %v, want one containing %q", problems, tc.wantSub)
			}
		})
	}
}

// scopedPlan is the smallest compliant plan with a ranked target and a layer matrix, so a test varies
// only what the scoped-run rule reads: the `Light:` declaration, whether the plan corroborates the
// target it names, and the reason a skipped layer carries.
func scopedPlan(light, scope string) string {
	return light +
		"## Findings\n\n" +
		"| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n\n" +
		"## Ranked targets\n\n" +
		"| Target | Blast radius | Churn / past fixes | Consequence class | Existing evidence | Altitude | Target rung |\n" +
		"|---|---|---|---|---|---|---|\n" +
		"| cli flags | `internal/cli/flags.go:12` | Unknown | incorrect output | | | |\n\n" +
		"## Layer matrix\n\n" +
		"| Layer | Skill | Scope | Status |\n|---|---|---|---|\n" +
		"| Persistence and migrations | `database-persistence-testing` | " + scope + " | n/a |\n"
}

const lightLine = "Light: cli flags · touches cli\n\n"

// The cheap half of the scoped-run decision is the declaration, and this is the one breach it adds:
// an `n/a` row in a plan that declares `Light:` has to say why the layer was left out. Whether the
// change was really bounded stays with the operator and the plan's reader.
func TestCheckLightPlanOwesAReasonForEverySkippedLayer(t *testing.T) {
	cases := []struct {
		name  string
		light string
		scope string
		want  string // the layer the breach must name; empty means the plan passes
	}{
		{
			name:  "a Light plan that skips a layer without a reason",
			light: lightLine,
			scope: "",
			want:  "Persistence and migrations",
		},
		{
			name:  "the same plan with a reason",
			light: lightLine,
			scope: "no persistence in the touched diff",
		},
		{
			name:  "a plan with no Light line is checked as it always was",
			light: "",
			scope: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", scopedPlan(tc.light, tc.scope))
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(problems, "\n")
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("compliant plan rejected: %v", problems)
				}
				return
			}
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("problems = %v, want one naming %q", problems, tc.want)
			}
			if !strings.Contains(joined, "Scope") {
				t.Fatalf("the breach must point at the Scope cell: %v", problems)
			}
		})
	}
}

// A `Light:` line is a claim the plan owes evidence for, not a header the check takes on faith: the
// shipped skeleton with the line added names a target the plan never ranked, so the check says so.
func TestCheckRejectsALightHeaderThePlanNeverRanked(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plan.md")
	if err := Init(p, false); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	scoped := strings.Replace(string(body), "Baseline:", lightLine+"Baseline:", 1)
	if scoped == string(body) {
		t.Fatal("the template header moved: the Light line was never placed")
	}
	if err := os.WriteFile(p, []byte(scoped), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(problems, "\n"); !strings.Contains(joined, "cli flags") {
		t.Fatalf("a Light header nothing corroborates must name the target it claims: %v", problems)
	}
}

// The declaration is the cheap half of the scoped-run decision, so the check reads its shape and that
// the plan corroborates the target it names. Whether the change was really bounded stays with the
// operator and the plan's reader, never with this check.
func TestCheckValidatesTheLightDeclaration(t *testing.T) {
	cases := []struct {
		name     string
		light    string
		want     string // a substring of the breach; empty means the plan passes
		wantOnly bool   // the breach must stand alone: one problem, no corroboration noise
	}{
		{
			name:  "a declaration whose target the plan ranked passes",
			light: lightLine,
		},
		{
			name:  "a target the plan cites as path:line corroborates itself",
			light: "Light: internal/cli/flags.go · touches cli\n\n",
		},
		{
			name:  "a declaration without the separator",
			light: "Light: cli flags\n\n",
			want:  "must read",
		},
		{
			name:     "a declaration whose blast radius is a placeholder",
			light:    "Light: n/a · touches cli\n\n",
			want:     "names no blast radius",
			wantOnly: true,
		},
		{
			name:     "a declaration whose touched classes are a placeholder",
			light:    "Light: cli flags · touches none\n\n",
			want:     "names no touched classes",
			wantOnly: true,
		},
		{
			name:  "a declaration with no touched classes",
			light: "Light: cli flags · touches  \n\n",
			want:  "names no touched classes",
		},
		{
			name:  "a target the plan never ranked and never cited",
			light: "Light: ledger rewrites · touches persistence\n\n",
			want:  "corroborates",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", scopedPlan(tc.light, "no persistence in the touched diff"))
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("compliant plan rejected: %v", problems)
				}
				return
			}
			if tc.wantOnly && len(problems) != 1 {
				t.Fatalf("problems = %v, want the one breach and nothing else", problems)
			}
			if joined := strings.Join(problems, "\n"); !strings.Contains(joined, tc.want) {
				t.Fatalf("problems = %v, want one containing %q", problems, tc.want)
			}
			if tc.want == "corroborates" && !strings.Contains(strings.Join(problems, "\n"), "ledger rewrites") {
				t.Fatalf("the breach must name the target it could not corroborate: %v", problems)
			}
		})
	}
}

// Activation is the narrow reading of a validated declaration. A detected defect, an ordinary plan, or
// a line that merely looks like a declaration is never activation; the bench records what this says.
func TestLightActivatedIsNarrow(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want bool
	}{
		{
			name: "a validated declaration activates",
			plan: scopedPlan(lightLine, "no persistence in the touched diff"),
			want: true,
		},
		{
			name: "no declaration never activates",
			plan: scopedPlan("", "no persistence in the touched diff"),
		},
		{
			name: "a declaration without the declared shape does not activate",
			plan: scopedPlan("Light: cli flags\n\n", "no persistence in the touched diff"),
		},
		{
			name: "an uncorroborated target does not activate",
			plan: scopedPlan("Light: ledger rewrites · touches persistence\n\n", "no persistence in the touched diff"),
		},
		{
			name: "a skipped layer with no reason does not activate",
			plan: scopedPlan(lightLine, ""),
		},
		{
			name: "prose that names the declaration is not a declaration",
			plan: "A plan that declares `Light:` owes a reason per layer.\n" + scopedPlan("", "no persistence in the touched diff"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LightActivated(tc.plan); got != tc.want {
				t.Fatalf("LightActivated = %v, want %v", got, tc.want)
			}
		})
	}
}

// A Light breach is located like every other row-level breach: a declaration that breaks its own shape
// names the line it sits on, so the reader opens the plan where the fix goes instead of grepping for the
// label the message quotes.
func TestCheckNamesTheLineOfTheLightDeclaration(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		light  string
		want   string
	}{
		{
			name:  "a declaration without the separator",
			light: "Light: cli flags\n\n",
			want:  "line 1: the Light declaration must read",
		},
		{
			name:  "a declaration whose blast radius is a placeholder",
			light: "Light: n/a · touches cli\n\n",
			want:  "line 1: the Light declaration names no blast radius",
		},
		{
			name:  "a declaration whose touched classes are a placeholder",
			light: "Light: cli flags · touches none\n\n",
			want:  "line 1: the Light declaration names no touched classes",
		},
		{
			name:  "a target the plan never ranked",
			light: "Light: ledger rewrites · touches persistence\n\n",
			want:  "line 1: the Light declaration names target ledger rewrites, which no Ranked-target row",
		},
		{
			name:   "a declaration below the plan header",
			prefix: "# Test plan\n\n",
			light:  "Light: ledger rewrites · touches persistence\n\n",
			want:   "line 3: the Light declaration names target ledger rewrites, which no Ranked-target row",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", tc.prefix+scopedPlan(tc.light, "no persistence in the touched diff"))
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			if joined := strings.Join(problems, "\n"); !strings.Contains(joined, tc.want) {
				t.Fatalf("problems = %v, want one containing %q", problems, tc.want)
			}
		})
	}
}

// The layer a Light plan left out is named by the line its own row sits on, not by the table that holds
// it: a second `n/a` row must not inherit the line of the first one.
func TestCheckNamesTheLineOfTheLayerItBlames(t *testing.T) {
	doc := scopedPlan(lightLine, "")
	// A compliant layer first, so the breach has a row above it to be confused with.
	doc = strings.Replace(doc, "| Persistence and migrations |", "| API contracts | `none` | reviewed in place | done |\n| Persistence and migrations |", 1)
	row := "| Persistence and migrations |"
	want := fmt.Sprintf("line %d: layer Persistence and migrations is out of scope", strings.Count(doc[:strings.Index(doc, row)], "\n")+1)
	p := write(t, t.TempDir(), "plan.md", doc)
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(problems, "\n"); !strings.Contains(joined, want) {
		t.Fatalf("problems = %v, want one containing %q", problems, want)
	}
}

func TestCheckOnAMissingFile(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Fatal("a missing plan must be an error, not a clean report")
	}
}

const header = "## Findings\n\n| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|---|\n"
const ledger = "\n## Evidence ledger\n\n| Id | Claim | Executed | Inputs | Observed | Mutation | Reproduction | Label |\n|---|---|---|---|---|---|---|---|\n"

// A breach that names a row has to say where the row is. `path: finding F1 cites no path:line` left the
// reader grepping the file for the row the check was talking about, which is the cost the checker
// exists to remove.
func TestCheckNamesTheLineOfTheRowItBlames(t *testing.T) {
	// `header` puts the column header on line 3 and the separator on line 4, so the first data row is
	// line 5: the number the breach has to carry.
	plan := header + "| F1 | `src/a.js:5` x | M | yes | E9 | t.js :: x | fixed | me | - | - |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
	p := write(t, t.TempDir(), "plan.md", plan)
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(problems, "\n"), "line 5: finding F1 cites evidence E9") {
		t.Fatalf("a row breach must name its line: %v", problems)
	}
}

// A blank line inside a table used to end the block: every row under it was dropped and the plan still
// reported `well formed`. A blank is a separator that lost its pipes, not the end of the table.
func TestCheckReadsARowSeparatedFromTheTableByABlankLine(t *testing.T) {
	// The blank sits on line 5; the row it used to hide is line 6.
	plan := header + "\n| F1 | `src/a.js:5` x | M | yes | E9 | t.js :: x | fixed | me | - | - |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
	p := write(t, t.TempDir(), "plan.md", plan)
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(problems, "\n"), "line 6: finding F1 cites evidence E9") {
		t.Fatalf("a row under a blank line is still a row: %v", problems)
	}
}

// The Findings table is not the only one the check reads back: a ledger row's breach names the ledger line,
// so the line anchor covers both tables the checker reads.
func TestCheckNamesTheLineOfTheLedgerRowItBlames(t *testing.T) {
	// The Findings row is line 5; the razonado ledger row is line 11.
	plan := header + "| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | razonado |\n"
	p := write(t, t.TempDir(), "plan.md", plan)
	problems, err := Check(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(problems, "\n"), "line 11: evidence E1 is labelled razonado") {
		t.Fatalf("a ledger breach must name its line: %v", problems)
	}
}

// The line anchor must not narrow the checker: this repository's own plan and both shipped fixtures are the
// ordinary valid plans it reads, and they stay well formed.
func TestCheckAcceptsTheShippedPlans(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "docs", "testing", "test-plan.md"),
		filepath.Join("..", "..", "assets", "skills", "test-strategy", "evals", "fixtures", "plans", "clean.md"),
		filepath.Join("..", "..", "assets", "skills", "test-strategy", "evals", "fixtures", "plans", "rejected.md"),
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if problems := CheckDocument(string(raw)); len(problems) != 0 {
				t.Fatalf("the unmodified plan is not well formed: %v", problems)
			}
		})
	}
}

// A backslash-escaped pipe is part of its cell. Splitting on it shifts every column to the right,
// which moves a layer's owner out of the column a report reads.
func TestSplitHonoursEscapedPipes(t *testing.T) {
	got := split(`| a \| b | ` + "`owner`" + ` | pending |`)
	want := []string{"a | b", "`owner`", "pending"} // the escape belongs to the syntax, the pipe to the cell
	if len(got) != len(want) {
		t.Fatalf("cells = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// The Findings vocabulary is closed. A status outside it used to be read by a `confirmed|fixed` regex
// and silently treated as unsettled, so a row labelled `resolved` stopped owing the test that holds its
// verdict and the plan still passed.
func TestCheckNamesAStatusOutsideTheFindingsVocabulary(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		pin     string
		want    string
		wantNot string
	}{
		{
			name:   "a status the vocabulary does not carry",
			status: "resolved",
			pin:    "",
			want:   `line 5: finding F1 status "resolved" is not one of: open, confirmed, fixed, rejected, wontfix`,
			// The breach is the vocabulary's. An unknown status is neither valid nor read as a settled one
			// that owes a pinning test, so the row is judged once and on the rule it actually broke.
			wantNot: "names no pinning test",
		},
		{
			name:   "a row with no status at all",
			status: "",
			pin:    "",
			want:   "line 5: finding F1 has no status: one of open, confirmed, fixed, rejected, wontfix",
		},
		{
			name:   "a documented settled status still owes its pinning test",
			status: " FIXED ",
			pin:    "",
			want:   "line 5: finding F1 is settled but names no pinning test",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := header + "| F1 | `src/a.js:5` x | M | yes | E1 | " + tc.pin + " | " + tc.status + " | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
			p := write(t, t.TempDir(), "plan.md", plan)
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("problems = %v, want one containing %q", problems, tc.want)
			}
			if tc.wantNot != "" && strings.Contains(joined, tc.wantNot) {
				t.Fatalf("problems = %v, want none containing %q", problems, tc.wantNot)
			}
		})
	}
}

// A closed vocabulary has an accepted side worth pinning too: every documented status is read as
// written, and a settled one that names its pinning test owes nothing.
func TestCheckAcceptsEveryDocumentedFindingsStatus(t *testing.T) {
	for _, status := range []string{"open", "confirmed", "fixed", "rejected", "wontfix"} {
		t.Run(status, func(t *testing.T) {
			plan := header + "| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | " + status + " | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
			p := write(t, t.TempDir(), "plan.md", plan)
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 0 {
				t.Fatalf("status %q is documented and owes nothing here: %v", status, problems)
			}
		})
	}
}

// A line that is not a table row closes the block, and a `|` row after it proves the table was cut in two.
func TestCheckReportsATableInterruptedByProse(t *testing.T) {
	plan := header +
		"| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
		"a sentence that closes the table\n" +
		"| F2 | `src/b.js:9` y | M | yes | E9 | t.js :: y | fixed | me | - | - |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
	joined := strings.Join(CheckDocument(plan), "\n")
	if !strings.Contains(joined, "line 6: the Findings table is interrupted at line 6") ||
		!strings.Contains(joined, "a sentence that closes the table") || !strings.Contains(joined, "line 7") {
		t.Fatalf("an interrupted table must name the cut, the line that made it and the row it hid:\n%s", joined)
	}
	if strings.Contains(joined, "cites evidence E9") {
		t.Fatalf("a row under an interruption was never read and must not be judged:\n%s", joined)
	}
}

// A `###` after a table header is a boundary only when a table of its own starts there, never a silent cut.
func TestCheckReportsATableInterruptedByASubheading(t *testing.T) {
	plan := header +
		"| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
		"### Notes\n" +
		"| F2 | `src/b.js:9` y | M | yes | E9 | t.js :: y | fixed | me | - | - |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
	joined := strings.Join(CheckDocument(plan), "\n")
	if !strings.Contains(joined, "line 6: the Findings table is interrupted at line 6") ||
		!strings.Contains(joined, "### Notes") || !strings.Contains(joined, "line 7") {
		t.Fatalf("a subheading that cuts the table must be reported as the cut it is:\n%s", joined)
	}
	if strings.Contains(joined, "cites evidence E9") {
		t.Fatalf("a row under an interruption was never read and must not be judged:\n%s", joined)
	}
}

// A fenced example above the real table must not become the Findings header.
func TestCheckIgnoresAFencedExampleTable(t *testing.T) {
	for _, marker := range []string{"```", "~~~"} {
		t.Run(marker, func(t *testing.T) {
			doc := "## Findings\n\n" + marker + "markdown\n| Id | Finding | Severity |\n|---|---|---|\n" +
				"| F9 | `src/fenced.ts:1` | visible error |\n" + marker + "\n\n" +
				strings.TrimPrefix(header, "## Findings\n\n") +
				"| F1 | `src/real.ts:1` | visible error | yes | E1 |  | resolved | me | - | - |\n" +
				ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
			joined := strings.Join(CheckDocument(doc), "\n")
			if !strings.Contains(joined, `finding F1 status "resolved"`) {
				t.Fatalf("the real row's breach is what the checker owes:\n%s", joined)
			}
			if strings.Contains(joined, "F9") || strings.Contains(joined, "interrupted") {
				t.Fatalf("the fenced example must not be read as a table:\n%s", joined)
			}
		})
	}
}

// An unclosed fence skipped every line under it: fail closed rather than read a shorter plan as a whole one.
func TestCheckReportsAnUnclosedFence(t *testing.T) {
	doc := header + "\n```markdown\n| Id | Finding |\n|---|---|\n"
	want := "line 6: the Findings table region ends inside a code fence opened at line 6, so nothing below it was read"
	if problems := CheckDocument(doc); !strings.Contains(strings.Join(problems, "\n"), want) {
		t.Fatalf("problems = %v, want one containing %q", problems, want)
	}
}

// A subheading is a boundary when its own table starts there, as the skeleton's `### Hypotheses` does.
func TestCheckAcceptsASubheadingThatOpensItsOwnTable(t *testing.T) {
	nested := header +
		"| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" +
		"### Notes\n\nNotes prose.\n\n| Note | Why |\n|---|---|\n| a note | a reason |\n" +
		ledger + "| E1 | c | cmd | i | o | m | r | observado |\n"
	if problems := CheckDocument(nested); len(problems) != 0 {
		t.Fatalf("a subheading with its own header and delimiter opens a table: %v", problems)
	}
	body, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	if problems := CheckDocument(body); len(problems) != 0 {
		t.Fatalf("the shipped template must stay well formed: %v", problems)
	}
}
