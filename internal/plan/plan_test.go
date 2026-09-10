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

func TestCheckOnAMissingFile(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Fatal("a missing plan must be an error, not a clean report")
	}
}

const header = "## Findings\n\n| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |\n|---|---|---|---|---|---|---|---|---|---|\n"
const ledger = "\n## Evidence ledger\n\n| Id | Claim | Executed | Inputs | Observed | Mutation | Reproduction | Label |\n|---|---|---|---|---|---|---|---|\n"

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
