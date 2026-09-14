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
		[]plan.LedgerRow{row("E1", "go test ./...", Digest("out\n"), "observado")},
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
	fresh := Digest(output)
	other := Digest("something else\n")

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
			if len(calls) != len(tc.wantRun) {
				t.Fatalf("runner saw %v, want %v", calls, tc.wantRun)
			}
			for i, command := range tc.wantRun {
				if calls[i].command != command || calls[i].dir != tc.opts.Dir {
					t.Fatalf("run %d = %#v, want command %q in dir %q", i, calls[i], command, tc.opts.Dir)
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
			if got := Digest(tc.left) == Digest(tc.right); got != tc.same {
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
		if got := Digest(in); got != empty {
			t.Fatalf("Digest(%q) = %q, want the sha256 of no bytes: %q", in, got, empty)
		}
	}
	if got, again := Digest("alpha\n"), Digest("alpha\n"); got != again {
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
			fresh := Digest("out\n")
			got, calls := admit(t, []plan.LedgerRow{row("E1", tc.cell, fresh, "observado")}, Options{Execute: true}, "out\n", nil)
			assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: tc.want, digest: fresh, lines: 1}})
			if len(calls) != 1 || calls[0].command != tc.want {
				t.Fatalf("runner saw %v, want the stripped command %q", calls, tc.want)
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
		"| E1 | the plan package passes | prose | `go test ./internal/plan` | none | ok | " + Digest(output) + " | reverted → red | rerun it | observado |\n"
	rows := plan.Ledger(doc)
	if len(rows) != 1 {
		t.Fatalf("ledger = %#v, want the one row", rows)
	}
	got, calls := admit(t, rows, Options{Execute: true, Dir: t.TempDir()}, output, nil)
	assertRows(t, got, []want{{id: "E1", verdict: VerdictAdmitted, command: "go test ./internal/plan", digest: Digest(output), lines: 1}})
	if len(calls) != 1 || calls[0].command != "go test ./internal/plan" {
		t.Fatalf("runner saw %v, want the admit cell with its one wrapping span stripped", calls)
	}
}
