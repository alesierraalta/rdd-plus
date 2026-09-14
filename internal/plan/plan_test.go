package plan

import (
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

// replaceFixture splices row in after the exact template literal old. A bare strings.Replace over a
// template literal has no guard of its own: when the shipped template drifts, the Replace no-ops and
// the test goes vacuously green while asserting nothing. This fails loudly instead, naming the drift
// and the fixture that has to follow it.
func replaceFixture(t *testing.T, doc, what, old, row string) string {
	t.Helper()
	out := strings.Replace(doc, old, old+row, 1)
	if out == doc {
		t.Fatalf("the template drifted: %s no longer holds the literal this fixture splices into, so the fixture must be updated to track the template (literal was %q)", what, old)
	}
	return out
}

func TestCheckAcceptsACompliantPlan(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plan.md")
	if err := Init(p, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(p)
	plan := replaceFixture(t, string(body), "the Findings header",
		"| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Pinning test (suite path :: test name) | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |\n|---|---|---|---|---|---|---|---|---|---|\n",
		"| F1 | `src/a.js:5` drops a quoted comma | data loss | yes | E1 | tests/a.test.js :: keeps a quoted comma | fixed | me / 2026-09-10 | - | abc123 |\n")
	plan = replaceFixture(t, plan, "the Evidence ledger header",
		"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Normalize | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n|---|---|---|---|---|---|---|---|---|---|---|\n",
		"| E1 | it drops the comma | `node --test` | node --test | `a,\"b,c\"` | 3 fields | sha256:7eada7a897497315d39d2541f5058a9631e80828245781b3c9c96205d9d759ed | | reverted → red | same input | observado |\n")
	if plan == string(body) {
		t.Fatal("the fixture replaced nothing, so this test would read the untouched template as a compliant plan")
	}
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

// A data row whose cell count disagrees with its own table's header was cut by an unescaped `|`, so every
// column to the right of the cut shifts and the row declares one thing while carrying another. Both
// directions earn a breach; the separator and placeholder rows table already skips earn none.
func TestCheckReportsRowsWhoseCellsDoNotMatchTheirHeader(t *testing.T) {
	const finding = "| F1 | `src/a.js:5` x | M | yes | E1 | t.js :: x | fixed | me | - | - |\n"
	const ledgerRow = "| E1 | c | cmd | i | o | m | r | observado |\n"
	cases := []struct {
		name     string
		plan     string
		wantSubs []string // every substring the breach must carry; empty means the plan passes
	}{
		{
			name: "a compliant plan owes no breach",
			plan: header + finding + ledger + ledgerRow,
		},
		{
			name:     "a row with more cells than its header",
			plan:     header + "| F1 | `src/a.js:5` gate|sync | M | yes | E1 | t.js :: x | fixed | me | - | - |\n" + ledger + ledgerRow,
			wantSubs: []string{"Findings", "F1", "11 cells", "header's 10", "unescaped"},
		},
		{
			name:     "a row with fewer cells than its header",
			plan:     header + "| F1 | `src/a.js:5` x | M | yes |\n" + ledger + ledgerRow,
			wantSubs: []string{"Findings", "F1", "4 cells", "header's 10"},
		},
		{
			name: "a separator row is not a data row",
			plan: header + finding + "|---|---|---|---|\n" + ledger + ledgerRow,
		},
		{
			name: "a placeholder row is not a data row",
			plan: header + finding + "| - | | |\n" + ledger + ledgerRow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", tc.plan)
			problems, err := Check(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.wantSubs) == 0 {
				if len(problems) != 0 {
					t.Fatalf("compliant plan rejected: %v", problems)
				}
				return
			}
			joined := strings.Join(problems, "\n")
			for _, want := range tc.wantSubs {
				if !strings.Contains(joined, want) {
					t.Fatalf("problems = %v, want one containing %q", problems, want)
				}
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

func TestCheckOnAMissingFile(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Fatal("a missing plan must be an error, not a clean report")
	}
}

// The repository's own plan is the file this check exists to keep honest. It is read through the same
// os.ReadFile path Check uses, and the only excuse for passing without reading it is that it is genuinely
// absent, so a missing file can never masquerade as a clean report.
func TestRepositoryPlanIsWellFormed(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "testing", "test-plan.md")
	if _, err := os.ReadFile(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("the repository plan %s is absent, so there is no checked copy to assert against", path)
		}
		t.Fatal(err)
	}
	problems, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("the repository's own plan reports breaches: %v", problems)
	}
}

const header = "## Findings\n\n| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|---|\n"
const ledger = "\n## Evidence ledger\n\n| Id | Claim | Executed | Inputs | Observed | Mutation | Reproduction | Label |\n|---|---|---|---|---|---|---|---|\n"

// The Evidence ledger is a table a machine reads, so its machine columns are located by name. This is
// the header the template ships: `Admit` and `Digest` are the two new cells, and neither shares a
// substring with the columns around it.
const machineHeader = "## Evidence ledger\n\n" +
	"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
	"|---|---|---|---|---|---|---|---|---|---|\n"

// machineColumns lists the names Ledger resolves, in the order it resolves them.
var machineColumns = []string{"id", "claim", "executed", "admit", "inputs", "observed", "digest", "mutation", "reproduction", "label"}

// Every field is filled with a value no other column shares, so a field resolved to the wrong column
// cannot pass.
func TestLedgerResolvesTheColumnsOfTheShippedHeader(t *testing.T) {
	doc := machineHeader +
		"| E1 | it keeps the comma | prose saying what was done | `node --test t.js` | `a,\"b,c\"` | 3 fields | sha256:1111 | reverted → red | rerun the command above | observado |\n"
	got := Ledger(doc)
	want := LedgerRow{
		ID: "E1", Claim: "it keeps the comma", Executed: "prose saying what was done",
		Admit: "`node --test t.js`", Inputs: "`a,\"b,c\"`", Observed: "3 fields",
		Digest: "sha256:1111", Mutation: "reverted → red",
		Reproduction: "rerun the command above", Label: "observado",
		Cells: 10, HeaderCells: 10,
	}
	if len(got) != 1 {
		t.Fatalf("rows = %#v, want the one ledger row", got)
	}
	if got[0] != want {
		t.Fatalf("row = %#v, want %#v", got[0], want)
	}
}

// Substring matching is all columnIndex knows, so the collisions are pinned: `Admit` must not resolve
// to `Executed`, `Digest` must not resolve to another column, and no two names may share a column.
func TestLedgerColumnNamesResolveToDistinctColumns(t *testing.T) {
	_, header := table(section(machineHeader, "Evidence ledger"))
	if header == nil {
		t.Fatal("the shipped header has no table")
	}
	seen := map[int]string{}
	for _, name := range machineColumns {
		i := columnIndex(header, name)
		if i < 0 {
			t.Fatalf("columnIndex(%q) = -1: the header stopped naming that column", name)
		}
		if other, dup := seen[i]; dup {
			t.Fatalf("%q and %q both resolve to column %d", name, other, i)
		}
		seen[i] = name
	}
	if a, e := columnIndex(header, "admit"), columnIndex(header, "executed"); a == e {
		t.Fatalf("admit resolved to the executed column %d", a)
	}
	if d, e := columnIndex(header, "digest"), columnIndex(header, "executed"); d == e {
		t.Fatalf("digest resolved to the executed column %d", d)
	}
}

// The old header ships no machine columns. A plan written before them still resolves every cell it
// has, and the two new fields stay empty rather than borrowing a neighbour.
func TestLedgerOnTheOldHeaderLeavesTheNewColumnsEmpty(t *testing.T) {
	got := Ledger(ledger + "| E1 | c | cmd | i | o | m | r | observado |\n")
	want := LedgerRow{ID: "E1", Claim: "c", Executed: "cmd", Inputs: "i", Observed: "o", Mutation: "m", Reproduction: "r", Label: "observado", Cells: 8, HeaderCells: 8}
	if len(got) != 1 {
		t.Fatalf("rows = %#v, want the one ledger row", got)
	}
	if got[0] != want {
		t.Fatalf("row = %#v, want %#v", got[0], want)
	}
}

// ID is the first cell and Label the last, the same reading Check does, so extra cells do not move
// them and a truncated row reads its last written cell as the label.
func TestLedgerKeepsTheFirstCellAsIDAndTheLastAsLabel(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want LedgerRow
	}{
		{
			name: "extra cells",
			doc:  machineHeader + "| E1 | c | prose | `run x` | i | o | sha256:aa | m | r | observado | extra one | extra two |\n",
			want: LedgerRow{ID: "E1", Claim: "c", Executed: "prose", Admit: "`run x`", Inputs: "i", Observed: "o", Digest: "sha256:aa", Mutation: "m", Reproduction: "r", Label: "extra two", Cells: 12, HeaderCells: 10},
		},
		{
			name: "missing cells",
			doc:  machineHeader + "| E1 | c | `run x` | `run x` | i |\n",
			want: LedgerRow{ID: "E1", Claim: "c", Executed: "`run x`", Admit: "`run x`", Inputs: "i", Label: "i", Cells: 5, HeaderCells: 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Ledger(tc.doc)
			if len(got) != 1 {
				t.Fatalf("rows = %#v, want the one ledger row", got)
			}
			if got[0] != tc.want {
				t.Fatalf("row = %#v, want %#v", got[0], tc.want)
			}
		})
	}
}

// Order is the document's, and the rows table already drops stay dropped: a separator line and a
// placeholder row are not conclusions.
func TestLedgerKeepsDocumentOrderAndSkipsWhatTheTableSkips(t *testing.T) {
	doc := machineHeader +
		"| E1 | first | prose | `run 1` | i | o | sha256:1 | m | r | observado |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n" +
		"| - | a placeholder row |  |  |  |  |  |  |  |  |\n" +
		"| E2 | second | prose | `run 2` | i | o | sha256:2 | m | r | observado |\n"
	got := Ledger(doc)
	if len(got) != 2 || got[0].ID != "E1" || got[1].ID != "E2" {
		t.Fatalf("rows = %#v, want E1 then E2 and nothing else", got)
	}
	if none := Ledger("## Findings\n\n| Id |\n|---|\n"); len(none) != 0 {
		t.Fatalf("a document with no Evidence ledger returned %#v", none)
	}
}

// recordRow is a ledger row whose Digest cell holds old digest, with a backslash-escaped pipe in
// three cells before it and two after it. A row that re-rendered its cells would rewrite those
// escapes, so the fixture is the proof the splice is by byte offset and not by re-joining.
const (
	oldDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	newDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func recordRow(digest string) string {
	return "| E1 | a claim with an escaped \\| pipe | prose a human reads | `run x` | `a\\|b` | observed \\| here | " + digest + " | mutation \\| negative | reproduction \\| again | observado |\n"
}

// RecordDigest is the one function in this package that writes: Check, Ledger and Gaps only read. It
// replaces the bytes of exactly one cell, so an escaped pipe anywhere in the row survives, and a cell
// the splice got wrong would move every column to its right.
func TestRecordDigestReplacesOnlyTheNamedCell(t *testing.T) {
	doc := machineHeader + recordRow(oldDigest)
	got, err := RecordDigest(doc, "E1", newDigest)
	if err != nil {
		t.Fatalf("RecordDigest = %v", err)
	}
	want := strings.Replace(doc, oldDigest, newDigest, 1)
	if got != want {
		t.Fatalf("RecordDigest rewrote bytes outside the digest cell:\ngot  %q\nwant %q", got, want)
	}
	if escaped := strings.Count(got, `\|`); escaped != 5 {
		t.Fatalf("the row has 5 escaped pipes (3 before the Digest cell, 2 after) and %d survived:\n%s", escaped, got)
	}
	if rows := Ledger(got); len(rows) != 1 || rows[0].Digest != newDigest {
		t.Fatalf("the ledger reads back %#v, want the fresh digest", rows)
	}
}

// A digest that was already recorded is the value the cell holds, so recording it again must be a
// no-op: a rerun of a recording command may not rewrite bytes it already wrote.
func TestRecordDigestIsIdempotent(t *testing.T) {
	doc := machineHeader + recordRow(oldDigest)
	once, err := RecordDigest(doc, "E1", newDigest)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := RecordDigest(once, "E1", newDigest)
	if err != nil {
		t.Fatal(err)
	}
	if twice != once {
		t.Fatalf("recording the same digest twice changed the document:\nonce  %q\ntwice %q", once, twice)
	}
}

// The Digest cell is located by name, so its position in the row cannot matter: a truncated row may
// carry it as the last cell.
func TestRecordDigestOnADigestCellThatIsTheLastCell(t *testing.T) {
	doc := machineHeader + "| E1 | the claim | prose | `run x` | none | the observation | " + oldDigest + " |\n"
	got, err := RecordDigest(doc, "E1", newDigest)
	if err != nil {
		t.Fatalf("RecordDigest = %v", err)
	}
	if want := strings.Replace(doc, oldDigest, newDigest, 1); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Every refusal is distinct and specific, so a caller can say what to change instead of guessing why
// nothing was written.
func TestRecordDigestRefusesEveryReasonItCannotWrite(t *testing.T) {
	noDigestColumn := "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs | Observed | Mutation | Reproduction | Label |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| E1 | the claim | prose | `run x` | none | the observation | reverted → red | rerun it | observado |\n"
	cases := []struct {
		name   string
		doc    string
		id     string
		digest string
		want   string // the refusal must name this
	}{
		{
			name: "the id is not a row in the ledger",
			doc:  machineHeader + recordRow(oldDigest), id: "E9", digest: newDigest,
			want: "is not a row in the Evidence ledger",
		},
		{
			name: "the header names no Digest column",
			doc:  noDigestColumn, id: "E1", digest: newDigest,
			want: "names no Digest column",
		},
		{
			name: "the row has fewer cells than the Digest column requires",
			doc:  machineHeader + "| E1 | the claim | `run x` |\n", id: "E1", digest: newDigest,
			want: "so the Digest column",
		},
		{
			name: "the digest is not a sha256 digest",
			doc:  machineHeader + recordRow(oldDigest), id: "E1", digest: "deadbeef",
			want: "is not a sha256 digest",
		},
		{
			name: "the document has no Evidence ledger section",
			doc:  "## Findings\n\n| Id |\n|---|\n", id: "E1", digest: newDigest,
			want: "no Evidence ledger section",
		},
		{
			name: "the Evidence ledger section holds no table",
			doc:  "## Evidence ledger\n\nno rows yet\n", id: "E1", digest: newDigest,
			want: "holds no table",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RecordDigest(tc.doc, tc.id, tc.digest)
			if err == nil {
				t.Fatalf("RecordDigest recorded into a plan it cannot record into, returning %q", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one naming %q", err, tc.want)
			}
			if got != "" {
				t.Fatalf("a refused record must return no document, got %q", got)
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

// The ledger's machine columns are resolved by name, and the row's own Normalize expression is one of them.
// A header that carries it must resolve it, and a header that does not must leave the field empty rather
// than borrowing the cell of the column beside it.
func TestLedgerResolvesTheNormalizeColumnByName(t *testing.T) {
	head := "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Normalize | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|\n"
	rows := Ledger(head + "| E1 | c | prose | go test ./... | i | o | sha256:aa | [0-9]+s | m | r | observado |\n")
	if len(rows) != 1 {
		t.Fatalf("ledger = %#v, want the one row", rows)
	}
	got := rows[0]
	if got.Normalize != "[0-9]+s" || got.Digest != "sha256:aa" || got.Admit != "go test ./..." || got.Label != "observado" {
		t.Fatalf("row = %#v, want the Normalize cell read as its own column", got)
	}
	if got.Cells != 11 || got.HeaderCells != 11 {
		t.Fatalf("row has %d cells against a %d-cell header, want 11 and 11", got.Cells, got.HeaderCells)
	}

	// The previous header has no Normalize column. The field stays empty, and every other column still
	// resolves to its own cell rather than shifting by one.
	old := "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n" +
		"| E1 | c | prose | go test ./... | i | o | sha256:aa | m | r | observado |\n"
	rows = Ledger(old)
	if len(rows) != 1 {
		t.Fatalf("ledger = %#v, want the one row", rows)
	}
	got = rows[0]
	if got.Normalize != "" || got.Digest != "sha256:aa" || got.Mutation != "m" || got.Reproduction != "r" || got.Label != "observado" {
		t.Fatalf("row = %#v, want no Normalize column with every other field still resolved", got)
	}

	// A row that lost a cell is reported by the count, not read from the wrong column. The document carries
	// only a ledger, so Check also reports the Findings section it does not have; the breach this asserts is
	// the cell count, named against the ledger.
	dir := t.TempDir()
	short := write(t, dir, "short.md", head+"| E1 | c | prose | go test ./... | i | o | sha256:aa | [0-9]+s | m | r |\n")
	problems, err := Check(short)
	if err != nil {
		t.Fatalf("Check(%s) failed: %v", short, err)
	}
	reported := false
	for _, p := range problems {
		if strings.Contains(p, "Evidence ledger") && strings.Contains(p, "11") {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("Check(%s) = %v, want the lost cell reported against the ledger", short, problems)
	}
}
