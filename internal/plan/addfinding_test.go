package plan

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

// addPlan is the smallest plan carrying one evidence row: a finding has something real to cite, and the
// checker has something to hold the candidate row to.
func addPlan(rows ...string) string {
	return header + strings.Join(rows, "") + ledger +
		"| E1 | it drops the comma | `node --test` | `a,b` | 3 fields | reverted -> red | same input | observado |\n"
}

// openFinding is a row the checker accepts as it stands, so each test varies exactly one thing.
func openFinding() Finding {
	return Finding{
		ID: "F1", Location: "src/a.js:5", Severity: "data loss", DataSafe: "yes", Evidence: "E1",
		Status: "open", VerdictBy: "me / 2026-09-10", Reason: "not pinned",
	}
}

// edited returns openFinding with one field changed, so a refusal test varies exactly one thing.
func edited(apply func(*Finding)) Finding {
	f := openFinding()
	apply(&f)
	return f
}

// add-finding exists so an operator stops hand-editing a ten-column row: it writes the row the checker
// already accepts and reports the line it landed on.
func TestAddFindingWritesARowTheCheckerAccepts(t *testing.T) {
	cases := []struct {
		name string
		plan string
	}{
		{"a table with no rows lands under the separator", addPlan()},
		{"a table with rows lands under the last one",
			addPlan("| F0 | `src/b.js:9` y | M | yes | E1 | t.js :: y | open | me | - | - |\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", tc.plan)
			line, err := AddFinding(p, openFinding())
			if err != nil {
				t.Fatalf("AddFinding: %v", err)
			}
			got, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if problems := CheckDocument(string(got)); len(problems) != 0 {
				t.Fatalf("the row add-finding wrote is rejected by its own checker: %v", problems)
			}
			lines := strings.Split(string(got), "\n")
			if line < 1 || line > len(lines) || !strings.HasPrefix(lines[line-1], "| F1 |") {
				t.Fatalf("the reported line %d is not the new row", line)
			}
		})
	}
}

// Every refusal leaves the file byte-identical. A command that half-writes a plan is worse than one that
// refuses, because the loss is silent: the runner closes on a plan that is missing the run.
func TestAddFindingRefusesWithoutMovingTheFile(t *testing.T) {
	cases := []struct {
		name    string
		plan    string
		finding Finding
		want    string
		usage   bool // the value refusals the CLI maps to exit 2
	}{
		{"a status outside the vocabulary", addPlan(), edited(func(f *Finding) { f.Status = "resolved" }),
			"open, confirmed, fixed, rejected, wontfix", true},
		{"a settled status with no pinning test", addPlan(), edited(func(f *Finding) { f.Status = "confirmed" }),
			"test", true},
		{"a location that is not path:line", addPlan(), edited(func(f *Finding) { f.Location = "src/a.js" }),
			"path:line", true},
		{"evidence that is not a ledger row", addPlan(), edited(func(f *Finding) { f.Evidence = "E1, E9" }),
			"E9", false},
		{"a value carrying a newline", addPlan(), edited(func(f *Finding) { f.Reason = "first\nsecond" }),
			"newline", true},
		{"a plan the checker already rejects",
			addPlan("| F0 | no path here | M | yes | E1 | | open | me | - | - |\n"), openFinding(),
			"would not pass plan check", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, t.TempDir(), "plan.md", tc.plan)
			before, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			_, err = AddFinding(p, tc.finding)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal %q does not name %q", err, tc.want)
			}
			if got := errors.Is(err, ErrUsage); got != tc.usage {
				t.Fatalf("errors.Is(err, ErrUsage) = %v, want %v: %q", got, tc.usage, err)
			}
			after, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a refusal must leave the plan byte-identical")
			}
		})
	}
}

// Applying the command twice refuses and changes nothing: two rows for one finding score one defect twice.
func TestAddFindingRefusesADuplicateID(t *testing.T) {
	p := write(t, t.TempDir(), "plan.md", addPlan())
	if _, err := AddFinding(p, openFinding()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddFinding(p, openFinding()); err == nil || !strings.Contains(err.Error(), "already row") {
		t.Fatalf("the second apply must refuse the duplicate, got %v", err)
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the refused second apply must not move the file")
	}
}

// The command writes findings only: the ledger an operator already audited stays exactly as it was.
func TestAddFindingWritesNoEvidenceLedgerRow(t *testing.T) {
	p := write(t, t.TempDir(), "plan.md", addPlan())
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddFinding(p, openFinding()); err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	before, after := strings.Split(string(original), "\n"), strings.Split(string(got), "\n")
	bh, be := sectionRegion(before, "Evidence ledger")
	ah, ae := sectionRegion(after, "Evidence ledger")
	if bh < 0 || ah < 0 {
		t.Fatal("the plan has no Evidence ledger")
	}
	if want, have := strings.Join(before[bh:be], "\n"), strings.Join(after[ah:ae], "\n"); want != have {
		t.Fatalf("the Evidence ledger moved:\nbefore:\n%s\nafter:\n%s", want, have)
	}
}
