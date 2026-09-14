package plan

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// setUserCacheDir points the per-user cache at dir for this process, so the plan lock directory under test is a
// test-owned one rather than the developer's real cache.
func setUserCacheDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LocalAppData", dir)
	t.Setenv("HOME", dir)
}

// Simultaneous calls used to lose rows: each one read the document, built its candidate and wrote it into place,
// so the losers were overwritten by the last write — and the plan still checked as `well formed`, which is what
// made the loss silent. The read, the validation and the write are one critical section held across processes,
// so every call's distinct id survives to the checker.
func TestAddFindingSerialisesConcurrentCalls(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "plan.md", addPlan())
	setUserCacheDir(t, t.TempDir())
	// The same plan reached through a relative and an absolute path is one file, so it has to be one lock: a lock
	// keyed on the spelling rather than on the file would serialise neither spelling against the other.
	t.Chdir(dir)

	ids := []string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"}
	start := make(chan struct{})
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			<-start
			f := openFinding()
			f.ID = id
			path := p
			if i%2 == 1 {
				path = filepath.Base(p)
			}
			_, errs[i] = AddFinding(path, f)
		}(i, id)
	}
	close(start)
	wg.Wait()

	for i, id := range ids {
		if err := errs[i]; err != nil {
			t.Fatalf("call %d adding %s: %v", i, id, err)
		}
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if n := strings.Count(string(got), "| "+id+" |"); n != 1 {
			t.Fatalf("finding %s appears %d times, want exactly once:\n%s", id, n, got)
		}
	}
	if problems := CheckDocument(string(got)); len(problems) != 0 {
		t.Fatalf("a plan every concurrent call contributed to is not well formed: %v", problems)
	}
	// The command writes findings only, however many callers ran at once: the ledger keeps the row it had.
	if ledger := scanSection(strings.Split(string(got), "\n"), "Evidence ledger"); len(ledger.rows) != 1 {
		t.Fatalf("the Evidence ledger has %d rows, want the one already there", len(ledger.rows))
	}
	// The lock lives outside the repository and the write leaves nothing behind: the plan's own directory holds
	// the plan and nothing else.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	if len(names) != 1 || names[0] != "plan.md" {
		t.Fatalf("the plan directory must hold the plan alone, found %v", names)
	}
}

// The write replaces the destination through a temp file and a rename, and the temp file carries whatever mode
// it was created with. A plan an operator tightened to 0600 must not come back 0644: the write has no licence
// to widen a file nobody asked it to widen.
func TestAddFindingKeepsThePlanMode(t *testing.T) {
	p := write(t, t.TempDir(), "plan.md", addPlan())
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AddFinding(p, openFinding()); err != nil {
		t.Fatalf("AddFinding: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("the plan was written mode %o, want the 600 the operator set", perm)
	}
}
