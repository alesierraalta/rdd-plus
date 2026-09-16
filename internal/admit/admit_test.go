package admit

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/plan"
)

// testLedgerHeader is the smallest ledger the reader accepts: the column names the row reader resolves by
// substring, with no Mode column, which is the plan a host-mode recording still writes.
const testLedgerHeader = "| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
	"|---|---|---|---|---|---|---|---|---|---|\n"

// testRow is one ledger row with an empty Digest cell for a recording run to fill.
func testRow(id, admit string) string {
	return "| " + id + " | the claim | prose a human reads | " + admit + " | none | the observation | | reverted → red | rerun it | observado |\n"
}

// writeTestPlan writes a plan document into a fresh directory and returns its path.
func writeTestPlan(t *testing.T, doc string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A dry run must reach no runner at all: what a caller sees in dry mode is a translation of the ledger, and
// the line per verdict plus the summary is the whole contract it can branch on. This is the seam the CLI
// wires to the real shell, so it is checked here on its own rather than only through the binary.
func TestRunDryRunReachesNoRunner(t *testing.T) {
	path := writeTestPlan(t, "## Evidence ledger\n\n"+testLedgerHeader+testRow("E1", "`printf 'one\\n'`")+testRow("E2", ""))
	calls := 0
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}

	code := Run(Request{Path: path, Dir: t.TempDir()}, Deps{
		Run: func(context.Context, string, string) (string, error) {
			calls++
			return "", nil
		},
		Out: out,
		Err: errOut,
	})

	if code != 1 {
		t.Fatalf("code = %d, want 1 (one row has no command to run)\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if calls != 0 {
		t.Fatalf("a dry run reached the runner %d times, want 0", calls)
	}
	if !strings.Contains(out.String(), "E1  WOULD RUN") {
		t.Fatalf("a row with a command and no digest is what a dry run would run:\n%s", out)
	}
	if !strings.Contains(out.String(), "[no-admit-command]") {
		t.Fatalf("a refused row prints its machine reason beside the sentence:\n%s", out)
	}
	if !strings.Contains(out.String(), "2 rows: 0 admitted, 1 would run, 1 refused, 0 recorded\n") {
		t.Fatalf("the summary counts every verdict:\n%s", out)
	}
}

// Run reads the plan before the rows run, and a row can take minutes. A plan another writer changed in the
// meantime must not be overwritten: the digest this run observed belongs to the document it read, so writing
// it over the newer file would erase that edit and pin a claim against a plan that no longer exists. The fake
// runner is the other writer here, which makes the race deterministic instead of timing-dependent.
func TestRunRefusesToWriteAPlanThatChangedUnderIt(t *testing.T) {
	path := writeTestPlan(t, "## Evidence ledger\n\n"+testLedgerHeader+testRow("E1", "`printf 'one\\n'`"))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}

	code := Run(Request{
		Path:    path,
		Execute: true,
		Record:  []string{"E1"},
		Dir:     t.TempDir(),
	}, Deps{
		Run: func(_ context.Context, _, _ string) (string, error) {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return "", err
			}
			defer f.Close()
			if _, err := f.WriteString("the other writer was here\n"); err != nil {
				return "", err
			}
			return "one\n", nil
		},
		Out: out,
		Err: errOut,
	})

	if code != 1 {
		t.Fatalf("code = %d, want 1 (the plan changed under the run)\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if !strings.Contains(errOut.String(), "changed while the rows ran") || !strings.Contains(errOut.String(), "nothing was written") {
		t.Fatalf("the refusal must say what happened and that nothing was written:\n%s", errOut)
	}
	if strings.Contains(out.String(), "RECORDED") {
		t.Fatalf("no row may be reported as recorded after the write was refused:\n%s", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "the other writer was here") {
		t.Fatalf("the other writer's edit must survive the refused run:\n%s", after)
	}
	if strings.Contains(string(after), "sha256:") {
		t.Fatalf("no digest may be written over a plan that changed under the run:\n%s", after)
	}
}

// A recording run writes the plan only after every row has run, so the comparison that refuses a changed plan and
// the write it guards are two separate steps: a writer landing between them was erased just the same. Every writer
// of the plan takes one lock, so the recording run has to take it too, and the observable difference is that a run
// holding no lock writes while another writer still holds it. This test pins that ordering: the lock is held until
// the run has reached its rows, and the release is recorded before it happens, so a run that returns before the
// release is a run that wrote unserialized. The grace period below only gives a correct run the moment it needs to
// attempt the lock; a run that never takes it is caught by the assertion, not by a stopwatch.
func TestRecordWaitsForThePlanLock(t *testing.T) {
	path := writeTestPlan(t, "## Evidence ledger\n\n"+testLedgerHeader+testRow("E1", "`printf 'one\\n'`"))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}

	lock, err := plan.LockPlan(path)
	if err != nil {
		t.Fatalf("taking the plan lock: %v", err)
	}
	rowRan := make(chan struct{})
	var rowSignal sync.Once
	released := make(chan struct{})
	go func() {
		<-rowRan
		time.Sleep(100 * time.Millisecond)
		close(released)
		plan.UnlockPlan(lock)
	}()

	code := Run(Request{
		Path:    path,
		Execute: true,
		Record:  []string{"E1"},
		Dir:     t.TempDir(),
	}, Deps{
		Run: func(_ context.Context, _, _ string) (string, error) {
			// The row machine may reach the runner more than once for one row; the signal is about the run
			// having reached the rows, not about the number of calls.
			rowSignal.Do(func() { close(rowRan) })
			return "one\n", nil
		},
		Out: out,
		Err: errOut,
	})

	select {
	case <-released:
	default:
		t.Fatalf("the run finished (exit %d) before the writer holding the plan lock let go, so an edit landing between the check and the write is still lost\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if code != 0 {
		t.Fatalf("once the lock is free the run must record the row: exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "sha256:") {
		t.Fatalf("the digest the run observed must be recorded once the lock is free:\n%s", after)
	}
}

// The plan a recording run writes back is the one write the ledger depends on, so it goes through the plan
// package's atomic write: a temp file in the directory and a rename. What an operator can see afterwards is what
// this test pins — the content is the whole document, the mode is the one the file already had, and no temp file
// survives the call — because a plan cut in half by a crash is a plan the next run refuses to read, and a temp
// file left behind is litter a later run trips over.
func TestWriteRecordedReplacesThePlanAtomically(t *testing.T) {
	doc := "## Evidence ledger\n\n" + testLedgerHeader + testRow("E1", "`printf 'one\\n'`")
	path := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeRecorded(path, []byte(doc), doc+"\n"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != doc+"\n" {
		t.Fatalf("the write did not replace the document: %q", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want the one the file already had", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("a temp file survived the atomic write: %v", entries)
		}
	}
}
