package evidence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/plan"
)

// row builds a ledger row with every machine cell filled, so a case varies only the cell it is about.
func row(id, admit, digest, label string) plan.LedgerRow {
	return plan.LedgerRow{
		ID: id, Claim: "the claim", Executed: "prose that says what was done",
		Admit: admit, Inputs: "the input", Observed: "the observation",
		Digest: digest, Mutation: "reverted → red", Reproduction: "rerun the command",
		Label: label,
	}
}

type call struct{ dir, command string }

// fakeRun records every command the admission asked for and returns canned output, so no test starts a
// process, touches the network, or writes a file.
func fakeRun(output string, err error, calls *[]call) func(context.Context, string, string) (string, error) {
	return func(_ context.Context, dir, command string) (string, error) {
		*calls = append(*calls, call{dir: dir, command: command})
		return output, err
	}
}

// fakeRuns returns each output in turn and repeats the last one, so a test can drive the stability
// probe: the first call is the observation, the second is the same command run again to see whether the
// output held still. Like fakeRun it starts no process, touches no network, and writes no file.
func fakeRuns(outputs []string, err error, calls *[]call) func(context.Context, string, string) (string, error) {
	seen := 0
	return func(_ context.Context, dir, command string) (string, error) {
		*calls = append(*calls, call{dir: dir, command: command})
		output := outputs[len(outputs)-1]
		if seen < len(outputs) {
			output = outputs[seen]
			seen++
		}
		return output, err
	}
}

// digest is Digest with the error a test cannot recover from flattened into a failure.
func digest(t *testing.T, output, normalize string) string {
	t.Helper()
	got, err := Digest(output, normalize)
	if err != nil {
		t.Fatalf("Digest(%q, %q) failed: %v", output, normalize, err)
	}
	return got
}

// want is the part of a RowResult a case asserts.
type want struct {
	id      string
	verdict Verdict
	reason  string
	command string
	detail  []string // substrings the human sentence must carry
	digest  string
	lines   int
}

func admit(t *testing.T, rows []plan.LedgerRow, opts Options, output string, err error) ([]RowResult, []call) {
	t.Helper()
	var calls []call
	return Admit(rows, opts, Deps{Run: fakeRun(output, err, &calls)}), calls
}

func assertRows(t *testing.T, got []RowResult, wants []want) {
	t.Helper()
	if len(got) != len(wants) {
		t.Fatalf("results = %#v, want %d rows", got, len(wants))
	}
	for i, w := range wants {
		r := got[i]
		if r.ID != w.id || r.Verdict != w.verdict || r.Reason != w.reason || r.Command != w.command || r.Digest != w.digest || r.Lines != w.lines {
			t.Fatalf("row %d = %#v, want id=%q verdict=%q command=%q reason=%q digest=%q lines=%d",
				i, r, w.id, w.verdict, w.command, w.reason, w.digest, w.lines)
		}
		for _, sub := range w.detail {
			if !strings.Contains(r.Detail, sub) {
				t.Fatalf("row %d detail = %q, want it to contain %q", i, r.Detail, sub)
			}
		}
	}
}

func assertNoRun(t *testing.T, calls []call) {
	t.Helper()
	if len(calls) != 0 {
		t.Fatalf("the admission reached the runner with %v, want no command run", calls)
	}
}

// Refusals decided from the row alone are decided before the runner is ever consulted: a row the
// ledger does not present as an observation, one with no command, and one whose command cannot be run
// as a single expanded command.
func TestAdmitRefusesFromTheRowAlone(t *testing.T) {
	cases := []struct {
		name string
		rows []plan.LedgerRow
		want []want
	}{
		{
			name: "a razonado row is a hypothesis, not an observation",
			rows: []plan.LedgerRow{row("E1", "go test ./...", "", "razonado")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "label-not-observado", detail: []string{"razonado"}}},
		},
		{
			name: "any other word than the literal observado is refused",
			rows: []plan.LedgerRow{row("E1", "go test ./...", "", "confirmed")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "label-not-observado", detail: []string{"confirmed"}}},
		},
		{
			name: "an empty label is refused and the detail names what it found",
			rows: []plan.LedgerRow{row("E1", "go test ./...", "", "")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "label-not-observado", detail: []string{"E1"}}},
		},
		{
			name: "an empty admit cell leaves nothing to run",
			rows: []plan.LedgerRow{row("E1", "  ", "", "observado")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "no-admit-command", detail: []string{"E1"}}},
		},
		{
			name: "a chained command is refused rather than guessed at",
			rows: []plan.LedgerRow{row("E1", "go test ./... ; go vet ./...", "", "observado")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "admit-multiple-commands"}},
		},
		{
			name: "an unexpanded placeholder is refused",
			rows: []plan.LedgerRow{row("E1", "go test <package>", "", "observado")},
			want: []want{{id: "E1", verdict: VerdictRefused, reason: "admit-has-placeholder", detail: []string{"<package>"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, calls := admit(t, tc.rows, Options{Execute: true, Dir: t.TempDir()}, "output\n", nil)
			assertRows(t, got, tc.want)
			assertNoRun(t, calls)
		})
	}
}

// An absent command is the empty cell and the placeholder tokens a plan author leaves behind. The
// match is the whole cell, so a command that merely contains one of these words still runs.
func TestAdmitTreatsAPlaceholderOnlyCellAsNoCommand(t *testing.T) {
	for _, token := range []string{"", "  ", "-", "—", "--", "n/a", "N/A", "na", "none", "tbd", "todo"} {
		t.Run(fmt.Sprintf("%q", token), func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", token, "", "observado")}, Options{Execute: true}, "", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "no-admit-command"}})
			assertNoRun(t, calls)
		})
	}
}

// Every shape that makes a cell more than one command is refused, because one row owes one stream of
// output to compare against one pinned digest.
func TestAdmitRefusesEveryShapeOfMoreThanOneCommand(t *testing.T) {
	forms := []string{
		"go test ./... ; go vet ./...",
		"go test ./... && go vet ./...",
		"go test ./... || true",
		"go test ./...\ngo vet ./...",
		"( go test ./... )",
		"{ go test ./...; }",
		"go test ./... &",
	}
	for _, cmd := range forms {
		t.Run(fmt.Sprintf("%q", cmd), func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", cmd, "", "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "admit-multiple-commands", detail: []string{"E1"}}})
			assertNoRun(t, calls)
		})
	}
}

// A placeholder is a value only the row's author can fill in: an angle span or a variable expansion.
func TestAdmitRefusesUnexpandedPlaceholders(t *testing.T) {
	forms := []struct{ cmd, placeholder string }{
		{"go test <package>", "<package>"},
		{"go test <tmp>", "<tmp>"},
		{"cat <file> > out.txt", "<file>"},
		{"test -d $HOME/work", "$HOME"},
		{"go test ${PACKAGE}", "${PACKAGE}"},
		{"echo ${ARTIFACT} > log", "${ARTIFACT}"},
	}
	for _, f := range forms {
		t.Run(f.cmd, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", f.cmd, "", "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "admit-has-placeholder", detail: []string{f.placeholder}}})
			assertNoRun(t, calls)
		})
	}
}

// The refusal rules must not eat a command that is one command: a shell-escaped variable, a quoted
// brace, and a flag that happens to spell a placeholder token all still run.
func TestAdmitRunsACommandThatIsOnlyOneCommand(t *testing.T) {
	for _, cmd := range []string{"go test ./... -count=1", `awk '{print $1}' input.txt`, "go test ./... -run none"} {
		t.Run(cmd, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", cmd, "", "observado")}, Options{Execute: false}, "", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictWouldRun, command: cmd, detail: []string{"not executed"}}})
			assertNoRun(t, calls)
		})
	}
}

// A dry run decides everything it can without observing anything: the row is runnable, so it is
// reported as such, and the runner is never called.
func TestAdmitDryRunNeverRunsAnything(t *testing.T) {
	called := false
	got := Admit(
		[]plan.LedgerRow{row("E1", "go test ./...", digest(t, "out\n", ""), "observado")},
		Options{Execute: false, Dir: t.TempDir()},
		Deps{Run: func(context.Context, string, string) (string, error) {
			called = true
			return "out\n", nil
		}},
	)
	if called {
		t.Fatal("a dry run must not start a process")
	}
	assertRows(t, got, []want{{id: "E1", verdict: VerdictWouldRun, command: "go test ./...", detail: []string{"E1", "not executed"}}})
}

// Once a row runs, the verdicts turn on the output alone: the failure, the deadline, silence, an
// unexpected digest, and a row that pins nothing each get their own reason.
func TestAdmitJudgesTheObservedOutput(t *testing.T) {
	const output = "alpha\nbeta\n"
	fresh := digest(t, output, "")
	other := digest(t, "something else\n", "")

	cases := []struct {
		name    string
		rows    []plan.LedgerRow
		opts    Options
		output  string
		runErr  error
		want    []want
		wantRun []string
	}{
		{
			name:    "the output matches the pinned digest",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", fresh, "observado")},
			opts:    Options{Execute: true},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./...", digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "a generic failure is a command failure and carries the error text",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", fresh, "observado")},
			opts:    Options{Execute: true},
			output:  "",
			runErr:  errors.New("exit status 1"),
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "command-failed", command: "go test ./...", detail: []string{"exit status 1"}}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "a deadline is a timeout, never a command failure",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", fresh, "observado")},
			opts:    Options{Execute: true, Timeout: 2 * time.Second},
			output:  "alpha\n",
			runErr:  fmt.Errorf("run go test: %w", context.DeadlineExceeded),
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "timeout", command: "go test ./...", detail: []string{"E1", "timeout"}}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "whitespace-only output is nothing to observe",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", fresh, "observado")},
			opts:    Options{Execute: true},
			output:  "  \n\t\n\n",
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "empty-output", command: "go test ./...", detail: []string{"E1"}}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "an unexpected digest names both digests",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", other, "observado")},
			opts:    Options{Execute: true},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "digest-mismatch", command: "go test ./...", detail: []string{other, fresh}, digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "a row that pins nothing is refused and names the record remedy",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", "", "observado")},
			opts:    Options{Execute: true},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "digest-missing", command: "go test ./...", detail: []string{"E1", "--record", fresh}, digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "recording replaces a pinned digest that no longer matches",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", other, "observado")},
			opts:    Options{Execute: true, Record: []string{"E1"}},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./...", digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "recording fills a digest that was never pinned",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", "", "observado")},
			opts:    Options{Execute: true, Record: []string{"E1"}},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./...", digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "recording another row does not admit this one",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", "", "observado")},
			opts:    Options{Execute: true, Record: []string{"E2"}},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "digest-missing", command: "go test ./...", digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "recording does not absolve a failed command",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", "", "observado")},
			opts:    Options{Execute: true, Record: []string{"E1"}},
			output:  "",
			runErr:  errors.New("exit status 1"),
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "command-failed", command: "go test ./..."}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "recording does not absolve silence",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", "", "observado")},
			opts:    Options{Execute: true, Record: []string{"E1"}},
			output:  " \n",
			want:    []want{{id: "E1", verdict: VerdictRefused, reason: "empty-output", command: "go test ./..."}},
			wantRun: []string{"go test ./..."},
		},
		{
			name:    "the runner gets the configured directory",
			rows:    []plan.LedgerRow{row("E1", "go test ./...", fresh, "observado")},
			opts:    Options{Execute: true, Dir: "/some/where"},
			output:  output,
			want:    []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./...", digest: fresh, lines: 2}},
			wantRun: []string{"go test ./..."},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, calls := admit(t, tc.rows, tc.opts, tc.output, tc.runErr)
			assertRows(t, got, tc.want)
			// A row refused at its first run never reaches the probe, so it is run once. Any row whose first
			// observation stood up is run a second time, because a pin over a moving output is not a pin.
			wantRuns := 2 * len(tc.wantRun)
			if len(tc.want) == 1 {
				switch tc.want[0].reason {
				case ReasonCommandFailed, ReasonTimeout, ReasonEmptyOutput:
					wantRuns = len(tc.wantRun)
				}
			}
			if len(calls) != wantRuns {
				t.Fatalf("runner saw %d calls %v, want %d", len(calls), calls, wantRuns)
			}
			for i, command := range tc.wantRun {
				if calls[i].command != command || calls[i].dir != tc.opts.Dir {
					t.Fatalf("run %d = %#v, want command %q in dir %q", i, calls[i], command, tc.opts.Dir)
				}
			}
			// The probe is the same command in the same directory as the observation it checks, or the two
			// outputs would not be comparable.
			for i := len(tc.wantRun); i < len(calls); i++ {
				if calls[i] != calls[i-len(tc.wantRun)] {
					t.Fatalf("probe run = %#v, want it identical to the observation it checks", calls[i])
				}
			}
		})
	}
}

// The timeout is the caller's bound, so it must reach the runner as a deadline, and a zero timeout
// must leave the command unbounded rather than inventing one.
func TestAdmitBoundsTheCommandWithTheConfiguredTimeout(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
		want    bool // whether the runner must receive a deadline
	}{
		{name: "a configured timeout becomes a deadline", timeout: 2 * time.Second, want: true},
		{name: "no timeout leaves the command unbounded", timeout: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var remaining time.Duration
			var bounded bool
			Admit([]plan.LedgerRow{row("E1", "go test ./...", "", "observado")}, Options{Execute: true, Timeout: tc.timeout}, Deps{
				Run: func(ctx context.Context, _, _ string) (string, error) {
					deadline, ok := ctx.Deadline()
					bounded = ok
					if ok {
						remaining = time.Until(deadline)
					}
					return "out\n", nil
				},
			})
			if bounded != tc.want {
				t.Fatalf("runner context bounded = %v, want %v", bounded, tc.want)
			}
			if tc.want && (remaining <= 0 || remaining > tc.timeout) {
				t.Fatalf("deadline left %s, want it inside %s", remaining, tc.timeout)
			}
		})
	}
}

// Only narrows the run to the rows named; everything else is left out of the result entirely rather
// than reported as skipped.
func TestAdmitOnlyKeepsTheNamedRows(t *testing.T) {
	rows := []plan.LedgerRow{
		row("E1", "go test ./...", "", "observado"),
		row("E2", "go vet ./...", "", "observado"),
	}
	got, calls := admit(t, rows, Options{Only: []string{"E2"}}, "", nil)
	assertRows(t, got, []want{{id: "E2", verdict: VerdictWouldRun, command: "go vet ./..."}})
	assertNoRun(t, calls)
	if none, _ := admit(t, rows, Options{Only: []string{"E9"}}, "", nil); len(none) != 0 {
		t.Fatalf("results = %#v, want no row at all", none)
	}
}

// Two outputs that differ only in trailing whitespace and trailing blank lines are the same
// observation; two that differ in content are not.
func TestDigestIgnoresTrailingWhitespaceAndBlankLines(t *testing.T) {
	cases := []struct {
		name  string
		left  string
		right string
		same  bool
	}{
		{name: "trailing spaces", left: "alpha\nbeta\n", right: "alpha  \nbeta\t\n", same: true},
		{name: "trailing blank lines", left: "alpha\nbeta\n", right: "alpha\nbeta\n\n\n", same: true},
		{name: "carriage returns", left: "alpha\nbeta\n", right: "alpha\r\nbeta\r\n\r\n", same: true},
		{name: "different content", left: "alpha\nbeta\n", right: "alpha\ngamma\n", same: false},
		{name: "a lost line", left: "alpha\nbeta\n", right: "alpha\n", same: false},
		{name: "moved content", left: "alpha\nbeta\n", right: "beta\nalpha\n", same: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := digest(t, tc.left, "") == digest(t, tc.right, ""); got != tc.same {
				t.Fatalf("Digest(%q) == Digest(%q) = %v, want %v", tc.left, tc.right, got, tc.same)
			}
		})
	}
}

// The digest is the one a caller can pin in the ledger: the sha256 of the normalized bytes, spelled
// as the ledger spells it. The empty output is pinned against the known sha256 of no bytes, so the
// normalization cannot quietly hash something else.
func TestDigestIsASha256OfTheNormalizedOutput(t *testing.T) {
	const empty = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	for _, in := range []string{"", "\n", "  \n\t\n"} {
		if got := digest(t, in, ""); got != empty {
			t.Fatalf("Digest(%q) = %q, want the sha256 of no bytes: %q", in, got, empty)
		}
	}
	if got, again := digest(t, "alpha\n", ""), digest(t, "alpha\n", ""); got != again {
		t.Fatalf("Digest is not stable: %q then %q", got, again)
	}
}

// The live ledger writes every command inside backticks, so a cell that is exactly one wrapping span
// is the ordinary spelling of a command and not markup: the wrapper is stripped and the inner command
// runs. The reported command and the runner both see the stripped text.
func TestAdmitStripsASingleWrappingSpan(t *testing.T) {
	cases := []struct{ cell, want string }{
		{"`go test ./...`", "go test ./..."},
		{"`go test ./... -count=1`", "go test ./... -count=1"},
		{"`  go test ./...  `", "go test ./..."},
	}
	for _, tc := range cases {
		t.Run(tc.cell, func(t *testing.T) {
			fresh := digest(t, "out\n", "")
			got, calls := admit(t, []plan.LedgerRow{row("E1", tc.cell, fresh, "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: tc.want, digest: fresh, lines: 1}})
			if len(calls) != 2 || calls[0].command != tc.want || calls[1] != calls[0] {
				t.Fatalf("runner saw %v, want the stripped command %q run twice identically", calls, tc.want)
			}
		})
	}
}

// `Executed` is prose a human reads and may quote with backticks; `Admit` carries one bare command and
// nothing else. A cell that uses a backtick any way other than one whole wrapping span is refused,
// because guessing which span is the command would be inventing an observation.
func TestAdmitRefusesBackticksThatAreNotOneWrappingSpan(t *testing.T) {
	cells := []string{
		"`go test ./...` and `go vet ./...`",
		"`go test ./...",
		"go test ./...`",
		"run `go test ./...`",
		"``go test ./...``",
	}
	for _, cell := range cells {
		t.Run(cell, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", cell, "", "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "admit-has-markup", detail: []string{cell, "Admit", "Executed"}}})
			assertNoRun(t, calls)
		})
	}
}

// The checks are owed in one order: label, absent cell, several commands, markup, placeholder. A cell
// that breaks more than one rule is refused for the earliest one, so the reason a run reports is the
// one the row's author has to fix first.
func TestAdmitRefusesForTheEarliestProblemFirst(t *testing.T) {
	cases := []struct{ name, cell, label, reason string }{
		{"a razonado row is refused before its cell is read", "`go test ./...` ; `go vet ./...`", "razonado", "label-not-observado"},
		{"several commands are refused before the markup around them", "`go test ./...` ; `go vet ./...`", "observado", "admit-multiple-commands"},
		{"markup is refused before the placeholder it hides", "run `go test <package>`", "observado", "admit-has-markup"},
		{"a clean span still has its placeholder checked", "`go test <package>`", "observado", "admit-has-placeholder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", tc.cell, "", tc.label)}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: tc.reason}})
			assertNoRun(t, calls)
		})
	}
}

// Every shell metacharacter that can sequence commands, start a second process, substitute a command,
// or redirect I/O is refused before the runner is reached: the row owes one command whose output is
// the observation, and the tool cannot tell what a second process or a file write would do.
func TestAdmitRefusesEveryShellMetacharacter(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		reason string
	}{
		{"a real newline starts a second command", "go test ./...\ngo vet ./...", "admit-multiple-commands"},
		{"a semicolon sequences a second command", "go test ./... ; go vet ./...", "admit-multiple-commands"},
		{"an ampersand backgrounds or joins", "go test ./... &", "admit-multiple-commands"},
		{"a pipe hands the stream to a second process", "printf 'abc' | tr a-z A-Z", "admit-multiple-commands"},
		{"a backtick substitutes a command", "echo `date`", "admit-has-markup"},
		{"a dollar-paren substitutes a command", "echo $(touch sentinel)", "admit-has-substitution"},
		{"an opening paren groups", "echo (", "admit-multiple-commands"},
		{"a closing paren groups", "echo )", "admit-multiple-commands"},
		{"an opening brace groups", "echo {a,b}", "admit-multiple-commands"},
		{"a closing brace groups", "echo a}", "admit-multiple-commands"},
		{"a less-than redirects input", "cat <sentinel", "admit-has-redirection"},
		{"a greater-than redirects output", "printf 'x' > sentinel.txt", "admit-has-redirection"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", tc.cmd, "", "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: tc.reason, detail: []string{"E1"}}})
			assertNoRun(t, calls)
		})
	}
}

// `$IDENT` stays legal: only `$(` substitutes a command. The shell reads a metacharacter inside
// quotes as literal text, so a quoted pipe or brace does not refuse the command either.
func TestAdmitKeepsQuotedMetacharactersAndPositionalParameters(t *testing.T) {
	for _, cmd := range []string{
		`awk '{print $1}' input.txt`,
		`grep -E 'a|b' input.txt`,
		`printf '%s\n' "$1"`,
	} {
		t.Run(cmd, func(t *testing.T) {
			got, calls := admit(t, []plan.LedgerRow{row("E1", cmd, "", "observado")}, Options{}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictWouldRun, command: cmd, detail: []string{"not executed"}}})
			assertNoRun(t, calls)
		})
	}
}

// The substitution rule keeps the positional parameter it must not eat: `awk '{print $1}'` runs and
// `$(touch x)` is refused, because a substitution is a second process the digest cannot name.
func TestAdmitRefusesCommandSubstitutionButKeepsPositionalParameters(t *testing.T) {
	refused, calls := admit(t, []plan.LedgerRow{row("E1", "$(touch x)", "", "observado")}, Options{Execute: true}, "out\n", nil)
	assertRows(t, refused, []want{{id: "E1", verdict: VerdictRefused, reason: "admit-has-substitution", detail: []string{"E1"}}})
	assertNoRun(t, calls)

	runnable, calls := admit(t, []plan.LedgerRow{row("E1", `awk '{print $1}' input.txt`, "", "observado")}, Options{}, "out\n", nil)
	assertRows(t, runnable, []want{{id: "E1", verdict: VerdictWouldRun, command: `awk '{print $1}' input.txt`}})
	assertNoRun(t, calls)
}

// A cell that redirects is refused before the shell sees it, so the file it would have created is
// never created: that side effect is exactly what the pinned digest cannot cover.
func TestAdmitRefusesARedirectBeforeTheShellCanWrite(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel.txt")
	got := Admit(
		[]plan.LedgerRow{row("E1", "printf 'x' > "+sentinel, "", "observado")},
		Options{Execute: true, Dir: dir},
		Deps{Run: func(_ context.Context, _, _ string) (string, error) {
			// A runner that would create the sentinel is the proof the refusal never reached one.
			if err := os.WriteFile(sentinel, []byte("ran"), 0o644); err != nil {
				t.Fatal(err)
			}
			return "x\n", nil
		}},
	)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "admit-has-redirection", detail: []string{"E1"}}})
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("the refused row still created %s, so a side effect escaped the digest", sentinel)
	}
}

// A row whose cells do not line up with the header's was cut by an unescaped `|`: the ledger reads
// one command and the shell would run another, so the row is refused before anything is run.
func TestAdmitRefusesARowThatDoesNotMatchTheHeader(t *testing.T) {
	const head = "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n"
	cases := []struct {
		name string
		row  string
		want []string
	}{
		{
			name: "an unescaped pipe split the Admit cell",
			row:  "| E1 | the claim | prose a human reads | printf 'abc' | tr a-z A-Z | none | the observation | sha256:aa | reverted → red | rerun it | observado |\n",
			want: []string{"E1", "11", "10", "unescaped", "|", "script"},
		},
		{
			name: "a truncated row has fewer cells than the header",
			row:  "| E1 | the claim | prose | printf 'abc' | observado |\n",
			want: []string{"E1", "5", "10", "unescaped"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := plan.Ledger(head + tc.row)
			if len(rows) != 1 {
				t.Fatalf("ledger = %#v, want the one row", rows)
			}
			got, calls := admit(t, rows, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "malformed-row", detail: tc.want}})
			assertNoRun(t, calls)
		})
	}
}

// The two halves compose: the rows Admit judges are the ones Ledger reads out of the document, and no
// process runs here either. The ledger wraps its command in backticks, and Admit strips that one
// wrapping span so the shell is handed the command itself rather than its quoting.
func TestAdmitReadsRowsThePlanLedgerParsed(t *testing.T) {
	const output = "ok rdd-plus/internal/plan\n"
	doc := "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n" +
		"| E1 | the plan package passes | prose | `go test ./internal/plan` | none | ok | " + digest(t, output, "") + " | reverted → red | rerun it | observado |\n"
	rows := plan.Ledger(doc)
	if len(rows) != 1 {
		t.Fatalf("ledger = %#v, want the one row", rows)
	}
	got, calls := admit(t, rows, Options{Execute: true, Dir: t.TempDir()}, output, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./internal/plan", digest: digest(t, output, ""), lines: 1}})
	// Two calls, not one: the second run is the stability probe, and it is handed the same command in the
	// same directory as the first, so the two observations are comparable.
	if len(calls) != 2 || calls[0].command != "go test ./internal/plan" || calls[1] != calls[0] {
		t.Fatalf("runner saw %v, want the admit cell with its one wrapping span stripped, run twice", calls)
	}
}

// admitRuns drives the admission with a runner that returns each output in turn, so a case can decide
// what the second run sees. No process runs here either.
func admitRuns(t *testing.T, rows []plan.LedgerRow, opts Options, outputs []string, err error) ([]RowResult, []call) {
	t.Helper()
	var calls []call
	return Admit(rows, opts, Deps{Run: fakeRuns(outputs, err, &calls)}), calls
}

// The defect this reproduces was measured, not imagined: `go test ./internal/plan -count=1` was run four
// times and produced four digests, and the only byte that moved was the elapsed time. A row pinned from
// any of those runs failed against the next one, so a pin over a moving output is not a pin at all.
func TestAdmitRefusesOutputThatChangesBetweenTwoRuns(t *testing.T) {
	rows := []plan.LedgerRow{{ID: "E1", Admit: "go test ./internal/plan", Label: "observado"}}
	got, calls := admitRuns(t, rows, Options{Execute: true}, []string{"ok\t0.056s\n", "ok\t0.059s\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "unstable-output",
		command: "go test ./internal/plan",
		detail:  []string{"sha256:", "Normalize", "deterministic"}}})
	if len(calls) != 2 {
		t.Fatalf("runner saw %d calls, want the two the probe owes", len(calls))
	}
}

// A stable row is admitted, and the runner is handed the same command in the same directory both times:
// the second run is only a probe if it is the same run.
func TestAdmitAdmitsAStableRowAndRunsItTwice(t *testing.T) {
	rows := []plan.LedgerRow{{ID: "E1", Admit: "printf one", Digest: digest(t, "one\n", ""), Label: "observado"}}
	got, calls := admitRuns(t, rows, Options{Execute: true, Dir: "/w"}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "printf one", digest: digest(t, "one\n", ""), lines: 1}})
	if len(calls) != 2 || calls[0] != calls[1] {
		t.Fatalf("runner saw %v, want one command run twice identically", calls)
	}
	if calls[0].dir != "/w" || calls[0].command != "printf one" {
		t.Fatalf("runner saw %v, want the row's command in the configured directory", calls)
	}
}

// The whole point of the Normalize column: a row that cannot hold still becomes pinnable once the part
// that moves is declared, and a third run whose duration differs again lands on the same pin.
func TestAdmitAdmitsAVolatileRowOnceNormalizeTamesIt(t *testing.T) {
	const duration = `[0-9]+\.[0-9]+s`
	rows := []plan.LedgerRow{{ID: "E1", Admit: "go test ./internal/plan", Normalize: duration, Label: "observado"}}
	got, _ := admitRuns(t, rows, Options{Execute: true, Record: []string{"E1"}}, []string{"ok\t0.056s\n", "ok\t0.059s\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./internal/plan",
		digest: digest(t, "ok\t0.056s\n", duration), lines: 1}})

	rows[0].Digest = digest(t, "ok\t0.056s\n", duration)
	got, _ = admitRuns(t, rows, Options{Execute: true}, []string{"ok\t9.999s\n", "ok\t1.234s\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./internal/plan",
		digest: digest(t, "ok\t9.999s\n", duration), lines: 1}})
}

// An expression that does not compile is a defect in the row, so it is reported before the row's command
// is ever spawned rather than after a run has been spent discovering it.
func TestAdmitRefusesARowWhoseNormalizeDoesNotCompile(t *testing.T) {
	rows := []plan.LedgerRow{{ID: "E1", Admit: "printf one", Normalize: "[", Label: "observado"}}
	got, calls := admitRuns(t, rows, Options{Execute: true}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "normalize-invalid", detail: []string{"["}}})
	assertNoRun(t, calls)
}

// Recording must not turn a moving output into a pin, or the next honest run would be refused by the
// tool's own mistake.
func TestRecordDoesNotPinAnUnstableRow(t *testing.T) {
	rows := []plan.LedgerRow{{ID: "E1", Admit: "printf one", Label: "observado"}}
	got, _ := admitRuns(t, rows, Options{Execute: true, Record: []string{"E1"}}, []string{"one\n", "two\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: "unstable-output",
		command: "printf one", detail: []string{"Normalize"}}})
}

// A digest over a rewritten output is a different digest from the raw one: the expression has to change
// what is hashed, or the column is decoration.
func TestDigestRewritesEveryMatchBeforeHashing(t *testing.T) {
	const duration = `[0-9]+\.[0-9]+s`
	if digest(t, "ok\t0.056s\n", duration) != digest(t, "ok\t9.999s\n", duration) {
		t.Fatal("a duration-normalized digest must not move when only the duration moved")
	}
	if digest(t, "ok\t0.056s\n", duration) == digest(t, "ok\t0.056s\n", "") {
		t.Fatal("a normalized digest must differ from the raw one, or the expression did nothing")
	}
	if digest(t, "a1b", "[0-9]") != digest(t, "aXb", "") {
		t.Fatal("every match must become a single X")
	}
}

func TestDigestRefusesAnExpressionThatDoesNotCompile(t *testing.T) {
	if _, err := Digest("x", "["); err == nil {
		t.Fatal("an expression that does not compile must be an error, not a silent pass")
	}
}

// The mode a pin was taken in is part of the pin, because the same command digests differently in a container
// than on this machine. A row pinned in one mode and checked in the other is refused for that reason, rather
// than for a digest mismatch that would say nothing about why the two disagree.
func TestAdmitRefusesARowPinnedInAnotherMode(t *testing.T) {
	pinned := plan.LedgerRow{ID: "E1", Admit: "printf one", Digest: digest(t, "one\n", ""), Mode: ModeHost, Label: "observado"}
	got, calls := admitRuns(t, []plan.LedgerRow{pinned}, Options{Execute: true, Mode: ModeSandbox}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: ReasonModeMismatch,
		detail: []string{"host", "sandbox"}}})
	assertNoRun(t, calls)

	pinned.Mode = ModeSandbox
	got, calls = admitRuns(t, []plan.LedgerRow{pinned}, Options{Execute: true, Mode: ModeHost}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: ReasonModeMismatch,
		detail: []string{"sandbox", "host"}}})
	assertNoRun(t, calls)

	// An empty cell means the host, which is where every pin taken before the mode existed was taken, so a row
	// written before the column still runs in the default mode.
	old := plan.LedgerRow{ID: "E1", Admit: "printf one", Digest: digest(t, "one\n", ""), Label: "observado"}
	got, _ = admitRuns(t, []plan.LedgerRow{old}, Options{Execute: true}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "printf one",
		digest: digest(t, "one\n", ""), lines: 1}})

	// Recording is how a row's mode gets set, so it is exempt from the comparison it would otherwise fail.
	got, calls = admitRuns(t, []plan.LedgerRow{old}, Options{Execute: true, Mode: ModeSandbox, Record: []string{"E1"}}, []string{"one\n"}, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "printf one",
		digest: digest(t, "one\n", ""), lines: 1}})
	if len(calls) != 2 {
		t.Fatalf("runner saw %d calls, want the two the probe owes", len(calls))
	}
}

// A row that declares a mutation is claiming its own command is falsifiable, and that claim is refused rather
// than admitted unchecked. A cell that does not parse, a file that is not there, a line that does not exist,
// text that is absent or ambiguous, and an edit that changes nothing are defects in the row, and a row whose
// claim cannot be replayed is refused because only a tree the caller owns can be edited and put back.
func TestAdmitRefusesARowWhoseMutationCannotBeChecked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "src.go"), []byte("package p\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate string
		reason string
	}{
		{"the cell does not parse", "no arrow", "mutation-malformed"},
		{"the edit changes nothing", "x => x @ src.go:3", "mutation-no-op"},
		{"the file is not there", "x => y @ nope.go:3", "mutation-not-found"},
		{"the line does not exist", "x => y @ src.go:99", "mutation-no-line"},
		{"the text is not on that line", "var := => var = @ src.go:1", "mutation-not-found"},
		{"a valid mutation still has no tree to replay it in", "x => y @ src.go:3", "mutation-not-replayed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := []plan.LedgerRow{{ID: "E1", Admit: "printf one", Mutate: tc.mutate, Label: "observado"}}
			got, calls := admitRuns(t, rows, Options{Execute: true, Dir: dir}, []string{"one\n"}, nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: tc.reason}})
			assertNoRun(t, calls)
		})
	}
}

// A runner that knows the failure is about its own environment rather than about the command says so with a
// Refusal, and that reason is reported as it stands: a sandbox that refused a write is not a failing test, and
// calling it one would send the reader looking in the wrong place.
func TestAdmitReportsWhatTheRunnerRefused(t *testing.T) {
	for _, reason := range []string{ReasonSandboxReadOnly, ReasonMisconfigured, ReasonNoNetwork} {
		t.Run(reason, func(t *testing.T) {
			calls := 0
			rows := []plan.LedgerRow{{ID: "E1", Admit: "printf one", Digest: digest(t, "one\n", ""), Label: "observado"}}
			got := Admit(rows, Options{Execute: true}, Deps{Run: func(context.Context, string, string) (string, error) {
				calls++
				return "the container said no", Refusal{Reason: reason, Detail: "the sandbox refused this row"}
			}})
			assertRows(t, got, []want{{id: "E1", verdict: VerdictRefused, reason: reason, command: "printf one",
				detail: []string{"the sandbox refused this row"}}})
			if calls != 1 {
				t.Fatalf("runner saw %d calls, want the one whose refusal ends the row", calls)
			}
		})
	}
}
