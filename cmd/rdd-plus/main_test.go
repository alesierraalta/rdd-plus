package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/buildinfo"
	"github.com/alesierraalta/rdd-plus/internal/evidence"
)

// buildCLI compiles the command once per test binary; the contract under test is the process's,
// not a function's, so it has to run as a process.
func buildCLI(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "rdd-plus")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func TestCLIContract(t *testing.T) {
	bin := buildCLI(t)
	cases := []struct {
		name     string
		args     []string
		stdin    string
		wantExit int
		wantOut  string // substring expected on stdout or stderr
	}{
		{name: "no command prints usage and exits 2", wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "unknown command prints usage and exits 2", args: []string{"bogus"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "bench with no subcommand exits 2", args: []string{"bench"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "bench with an unknown subcommand exits 2", args: []string{"bench", "bogus"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		// A runner the bench cannot spawn must be refused before it scaffolds or spawns anything.
		{name: "bench run with an unknown runner exits 2", args: []string{"bench", "run", "--runner", "gemini"}, wantExit: 2, wantOut: "use pi or claude"},
		{name: "bench score without arguments exits 2", args: []string{"bench", "score"}, wantExit: 2, wantOut: "needs --case"},
		{name: "bench score with both workspace and plan exits 2", args: []string{"bench", "score", "--case", "c", "--workspace", "w", "--plan", "p"}, wantExit: 2, wantOut: "exactly one"},
		{name: "bench compare without two directories exits 2", args: []string{"bench", "compare", "only-one"}, wantExit: 2, wantOut: "two result directories"},
		{name: "bench compare on a missing directory exits 1", args: []string{"bench", "compare", "/nonexistent-a", "/nonexistent-b"}, wantExit: 1, wantOut: "compare:"},
		{name: "bench rescore without a directory exits 2", args: []string{"bench", "rescore"}, wantExit: 2, wantOut: "one results directory"},
		{name: "bench rescore on a missing directory exits 1", args: []string{"bench", "rescore", "/nonexistent"}, wantExit: 1, wantOut: "rescore:"},
		{name: "an unknown flag on a subcommand exits 2", args: []string{"doctor", "--nope"}, wantExit: 2, wantOut: "flag provided but not defined"},
		{name: "check refuses a path outside the repository", args: []string{"check", "--path", "../outside.md"}, wantExit: 2, wantOut: "--path"},
		{name: "check refuses an absolute path", args: []string{"check", "--path", "/abs.md"}, wantExit: 2, wantOut: "--path"},
		{name: "plan with no subcommand exits 2", args: []string{"plan"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "plan with an unknown subcommand exits 2", args: []string{"plan", "bogus"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "plan check on a missing file exits 1", args: []string{"plan", "check", "--path", "/nonexistent/plan.md"}, wantExit: 1, wantOut: "plan check:"},
		{name: "plan gaps on a missing file exits 1", args: []string{"plan", "gaps", "--path", "/nonexistent/plan.md"}, wantExit: 1, wantOut: "plan gaps:"},
		{name: "plan admit on a missing file exits 1", args: []string{"plan", "admit", "--path", "/nonexistent/plan.md"}, wantExit: 1, wantOut: "plan admit:"},
		{name: "plan admit with an unreadable timeout exits 2", args: []string{"plan", "admit", "--timeout", "soon"}, wantExit: 2, wantOut: "invalid value"},
		// The gate is a hook: whatever it receives, it must not break the turn.
		{name: "gate on empty stdin exits 0", args: []string{"gate"}, stdin: "", wantExit: 0},
		{name: "gate on malformed stdin exits 0", args: []string{"gate"}, stdin: "{not json", wantExit: 0},
		{name: "gate on an unknown flag exits 0", args: []string{"gate", "--nope"}, stdin: "{}", wantExit: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			cmd.Stdin = strings.NewReader(tc.stdin)
			cmd.Env = append(os.Environ(), "TESTING_GATE_LOG="+filepath.Join(t.TempDir(), "gate.jsonl"))
			out, err := cmd.CombinedOutput()
			code := 0
			if ee := (&exec.ExitError{}); err != nil {
				if ok := asExit(err, ee); ok {
					code = ee.ExitCode()
				} else {
					t.Fatalf("run: %v", err)
				}
			}
			if code != tc.wantExit {
				t.Fatalf("exit = %d, want %d\n%s", code, tc.wantExit, out)
			}
			if tc.wantOut != "" && !strings.Contains(string(out), tc.wantOut) {
				t.Fatalf("output missing %q:\n%s", tc.wantOut, out)
			}
		})
	}
}

// ledgerHeader is the shipped Evidence ledger header. The `Admit` and `Digest` columns are the two a
// recording run reads and writes, so a fixture drifting from this header would test another contract.
const ledgerHeader = "| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |\n" +
	"|---|---|---|---|---|---|---|---|---|---|\n"

// writeLedger writes a minimal plan whose one Evidence ledger row carries the given Admit cell, so a
// test drives the real binary over a real file rather than a string it never parsed.
func writeLedger(t *testing.T, dir, admit string) string {
	t.Helper()
	path := filepath.Join(dir, "plan.md")
	doc := "## Evidence ledger\n\n" + ledgerHeader + ledgerRow("E1", admit)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ledgerRow is one ledger row with the given id and Admit cell and an empty Digest cell for a
// recording run to fill.
func ledgerRow(id, admit string) string {
	return "| " + id + " | the claim | prose a human reads | " + admit + " | none | the observation | | reverted → red | rerun it | observado |\n"
}

// compliantPlan is the smallest plan `plan check` accepts, so a recording run can be followed by a
// check that still says well formed rather than by a smaller file that merely holds a digest.
func compliantPlan(rows ...string) string {
	return "## Findings\n\n" +
		"| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n" +
		"| F1 | `src/a.js:5` drops a quoted comma | data loss | yes | E1 | tests/a.test.js :: keeps a comma | fixed | me / 2026-09-10 | - | abc123 |\n" +
		"\n## Evidence ledger\n\n" + ledgerHeader + strings.Join(rows, "")
}

var digestRe = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

// --record is the flag that turns an observation into a pinned value, so the one combination that
// would pin an observation this run never made is refused before the plan is read and before any
// command reaches the runner.
func TestPlanAdmitRecordRequiresExecute(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	path := writeLedger(t, dir, "touch "+sentinel)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, "plan", "admit", "--path", path, "--record", "E1")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "--record") || !strings.Contains(out, "--execute") {
		t.Fatalf("the usage error must name the flag that was given and the flag it needs:\n%s", out)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("a usage error ran the command anyway")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("a usage error wrote the plan:\n%s", after)
	}
}

// A recording run pins what it just observed and writes the plan once for the whole pass. The three
// rows also prove the composed edit: one write leaves both digests and no rewritten bytes.
func TestPlanAdmitRecordPinsTheObservedDigests(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	before := compliantPlan(
		ledgerRow("E1", "`printf 'one\\n'`"),
		ledgerRow("E2", "`printf 'two\\n'`"),
		ledgerRow("E3", "`printf 'three\\n'`"),
	)
	if err := os.WriteFile(path, []byte(before), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, "plan", "admit", "--path", path, "--execute", "--record", "E1,E2,E3")
	if code != 0 {
		t.Fatalf("recording run exit = %d, want 0\n%s", code, out)
	}
	if n := strings.Count(out, "RECORDED"); n != 3 {
		t.Fatalf("want one RECORDED line per row, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "3 recorded") {
		t.Fatalf("the summary must count the recorded rows:\n%s", out)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digests := digestRe.FindAllString(string(after), -1)
	if len(digests) != 3 {
		t.Fatalf("want three recorded digests, got %v:\n%s", digests, after)
	}
	want := before
	for _, d := range digests {
		want = strings.Replace(want, "| | reverted → red |", "| "+d+" | reverted → red |", 1)
	}
	if string(after) != want {
		t.Fatalf("the recording run rewrote bytes outside the digest cells:\ngot  %q\nwant %q", after, want)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("the run must preserve the plan's mode: %v %v", info.Mode(), err)
	}

	// The rows are runnable, not digest-missing: the dry run reports them as runnable, and an executed
	// run without --record now admits the same observation instead of refusing an unpinned row.
	out, code = runCLI(t, bin, "plan", "admit", "--path", path)
	if code != 0 || strings.Count(out, "WOULD RUN") != 3 || strings.Contains(out, "digest-missing") {
		t.Fatalf("dry run after recording = %d\n%s", code, out)
	}
	out, code = runCLI(t, bin, "plan", "admit", "--path", path, "--execute")
	if code != 0 || strings.Count(out, "ADMITTED") != 3 {
		t.Fatalf("an executed run after recording must admit = %d\n%s", code, out)
	}
	out, code = runCLI(t, bin, "plan", "check", "--path", path)
	if code != 0 || !strings.Contains(out, "well formed") {
		t.Fatalf("plan check after recording = %d\n%s", code, out)
	}
}

// A refusal to splice aborts the whole write rather than skipping the row: a half-recorded ledger is
// exactly the state this feature exists to prevent, so the file must come back byte-identical.
func TestPlanAdmitRecordAbortsTheWholeWriteWhenTheSpliceRefuses(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	before := "## Evidence ledger\n\n" +
		"| Id | Claim | Executed | Admit | Inputs | Observed | Mutation | Reproduction | Label |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| E1 | the claim | prose | `echo hi` | none | the observation | reverted → red | rerun it | observado |\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, "plan", "admit", "--path", path, "--execute", "--record", "E1")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "plan admit:") || !strings.Contains(out, "names no Digest column") {
		t.Fatalf("the refused splice must be reported on stderr with the plan admit: prefix:\n%s", out)
	}
	if strings.Contains(out, "RECORDED") {
		t.Fatalf("a refused splice must record nothing:\n%s", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Fatalf("a refused splice must leave the plan byte-identical:\ngot  %q\nwant %q", after, before)
	}
}

// A dry run is the default for a reason: the runner hands the plan's cell to a shell, so reading a
// ledger must not execute it. The sentinel file is the proof the command never reached one.
func TestPlanAdmitDryRunRunsNothing(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	path := writeLedger(t, dir, "touch "+sentinel)

	out, code := runCLI(t, bin, "plan", "admit", "--path", path)
	if code != 0 {
		t.Fatalf("dry run exit = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "WOULD RUN") {
		t.Fatalf("a dry run must report WOULD RUN:\n%s", out)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("the dry run executed the command: %s exists", sentinel)
	}
}

// A row with no Admit cell is refused by name, and a refusal is exit 1: one refused row is not a clean
// run, whatever the other rows did.
func TestPlanAdmitRefusesARowWithoutACommand(t *testing.T) {
	bin := buildCLI(t)
	path := writeLedger(t, t.TempDir(), "")

	out, code := runCLI(t, bin, "plan", "admit", "--path", path)
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "REFUSED") || !strings.Contains(out, "no-admit-command") {
		t.Fatalf("the refusal must be named on stdout:\n%s", out)
	}
}

// The binary accepts a whole ledger's worth of rows and re-reads the plan file rather than trusting an
// index: `--only` narrows the run. An id the ledger does not carry is a mistake, not a silent no-op,
// so `--only` refuses it the same way `--record` does.
func TestPlanAdmitOnlyKeepsTheNamedRows(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	path := writeLedger(t, dir, "touch "+filepath.Join(dir, "ran"))

	out, code := runCLI(t, bin, "plan", "admit", "--path", path, "--only", "E1")
	if code != 0 || !strings.Contains(out, "E1") || !strings.Contains(out, "WOULD RUN") {
		t.Fatalf("--only E1 = %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); !os.IsNotExist(err) {
		t.Fatal("a narrowed dry run must still run nothing")
	}
	out, code = runCLI(t, bin, "plan", "admit", "--path", path, "--only", "E9")
	if code != 2 || !strings.Contains(out, "E9") || !strings.Contains(out, "available ids") {
		t.Fatalf("--only must refuse an id the ledger does not carry = %d\n%s", code, out)
	}
}

// An id a flag names but the ledger does not carry is a mistake the user must see: it is reported on
// stderr with the plan admit: prefix, names the unknown id and the ids that do exist, and exits 2
// before the plan is touched or a command is run.
func TestPlanAdmitRefusesAnUnknownID(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	path := writeLedger(t, dir, "touch "+sentinel)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		flag string
		args []string
	}{
		{"an unknown --record id", "--record", []string{"plan", "admit", "--path", path, "--execute", "--record", "ZZZ"}},
		{"an unknown --only id", "--only", []string{"plan", "admit", "--path", path, "--only", "ZZZ"}},
		{"an unknown id beside a known one", "--only", []string{"plan", "admit", "--path", path, "--only", "E1,ZZZ"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runCLI(t, bin, tc.args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2\n%s", code, out)
			}
			for _, want := range []string{"plan admit:", "ZZZ", tc.flag, "available ids", "E1"} {
				if !strings.Contains(out, want) {
					t.Fatalf("the refusal must carry %q:\n%s", want, out)
				}
			}
		})
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("a usage error ran the command anyway")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("a usage error wrote the plan:\n%s", after)
	}
}

// An unescaped pipe in an Admit cell splits the row: `plan check` saw a well formed table, but the
// ledger read one command while the shell would have run another. The row is refused, not truncated.
func TestPlanAdmitRefusesAnUnescapedPipeInARow(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	path := filepath.Join(dir, "plan.md")
	row := "| E1 | the claim | prose | printf 'abc' | tr a-z A-Z > " + sentinel + " | none | the observation | | reverted → red | rerun it | observado |\n"
	if err := os.WriteFile(path, []byte("## Evidence ledger\n\n"+ledgerHeader+row), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, "plan", "admit", "--path", path, "--execute")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "malformed-row") {
		t.Fatalf("the truncated row must be refused as malformed:\n%s", out)
	}
	if strings.Contains(out, "WOULD RUN") || strings.Contains(out, "printf 'abc'") {
		t.Fatalf("a malformed row must not report a runnable command:\n%s", out)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("the truncated cell still ran and wrote its sentinel")
	}
}

func asExit(err error, target *exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = *ee
	}
	return ok
}

// The feedback command is the destination the gate's offer always lacked: --template prints a
// skeleton, --file records it, and no flags reads the reports back.
func TestFeedbackCLI(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()

	out, code := runCLI(t, bin, "feedback", "--config-dir", dir, "--template")
	if code != 0 {
		t.Fatalf("template exit = %d\n%s", code, out)
	}
	for _, want := range []string{"ts:", "repo:", "plan:", "skill:", "build:", "--file", "verdict"} {
		if !strings.Contains(out, want) {
			t.Fatalf("template missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "telemetry")); !os.IsNotExist(err) {
		t.Fatalf("--template must write nothing, found a telemetry directory")
	}

	// No subcommand is the cheapest path to the answer.
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir)
	if code != 0 || !strings.Contains(strings.ToLower(out), "no reports") {
		t.Fatalf("bare feedback = %d %q", code, out)
	}

	report := filepath.Join(t.TempDir(), "report.md")
	body := "ts: 2026-09-10T12:00:00Z\n" +
		"repo: " + dir + "\n" +
		"plan: docs/testing/test-plan.md\n" +
		"skill: 0.3.6\n" +
		"build: test\n" +
		"paid: it found the defect\n" +
		"cost: one hour\n" +
		"reason: it earned its keep\n" +
		"verdict: paid\n" +
		"guess: what a probe proves\n"
	if err := os.WriteFile(report, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir, "--file", report)
	if code != 0 {
		t.Fatalf("submit exit = %d\n%s", code, out)
	}
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir, "--summary")
	if code != 0 || !strings.Contains(out, "1 report") || !strings.Contains(out, "paid: 1") {
		t.Fatalf("summary after one report = %d\n%s", code, out)
	}

	bad := filepath.Join(t.TempDir(), "bad.md")
	if err := os.WriteFile(bad, []byte(body+"surprise: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir, "--file", bad)
	if code != 2 || !strings.Contains(out, "surprise") {
		t.Fatalf("unknown key = %d %q", code, out)
	}

	if err := os.WriteFile(bad, []byte(strings.Replace(body, "verdict: paid", "verdict: maybe", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir, "--file", bad)
	if code != 2 || !strings.Contains(out, "paid") || !strings.Contains(out, "partly") || !strings.Contains(out, "ceremony") {
		t.Fatalf("bad verdict = %d %q", code, out)
	}

	if err := os.WriteFile(bad, []byte(strings.Replace(body, "cost: one hour\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "feedback", "--config-dir", dir, "--file", bad)
	if code != 2 || !strings.Contains(out, "cost") {
		t.Fatalf("missing field = %d %q", code, out)
	}

	// Every refusal wrote nothing: the ledger still holds the one accepted report.
	raw, err := os.ReadFile(filepath.Join(dir, "telemetry", "run-feedback.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if rows := strings.Count(strings.TrimRight(string(raw), "\n"), "\n") + 1; rows != 1 {
		t.Fatalf("a refusal must write nothing, ledger has %d rows:\n%s", rows, raw)
	}
}

func runCLI(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var ee exec.ExitError
	if asExit(err, &ee) {
		return string(out), ee.ExitCode()
	}
	t.Fatalf("run %v: %v", args, err)
	return "", -1
}

// A usage text that does not list a command it accepts sends users to the wrong place.
func TestUsageListsEveryBenchSubcommand(t *testing.T) {
	for _, sub := range []string{"bench run", "bench score", "bench history", "bench compare", "bench rescore", "plan init", "plan check", "plan gaps", "plan add-finding", "plan admit"} {

		if !strings.Contains(usage, sub) {
			t.Errorf("usage does not document %q", sub)
		}
	}
	for _, cmd := range []string{"gate", "sync", "doctor", "bench", "plan", "feedback", "version"} {
		if !strings.Contains(usage, "  "+cmd+" ") {
			t.Errorf("usage does not document the %q command", cmd)
		}
	}
}

// The version command names the build: the release and the commit behind it. A bare literal
// would say nothing about which build is installed, so the shape is the contract. The release is
// derived from buildinfo.Version rather than repeated here: a version bump moves the binary and
// the expectation together, so the suite documents the shape without pinning a number to bump.
func TestVersionNamesTheBuild(t *testing.T) {
	bin := buildCLI(t)
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	want := regexp.MustCompile("^" + regexp.QuoteMeta(buildinfo.Version) + ` \(([0-9a-f]{7}(\+dirty)?|unknown)\)$`)
	if !want.MatchString(got) {
		t.Fatalf("version printed %q, want %s", got, want)
	}
}

// The probe runs the wired Stop command, so a command it cannot read must be refused before anything is
// run: an unterminated quote used to be an error the probe turned into the doctor's own verdict, and it
// must not become an attempt to execute the fragment. The split-rule cases that used to live here belong
// to the one splitter, doctor.ShellWords, and are tested in internal/doctor.
func TestProbeHookRefusesACommandItCannotRead(t *testing.T) {
	for _, command := range []string{"", "   \t", `"unbalanced`, `'unbalanced`, `/h/bin/rdd-plus "gate`} {
		err := probeHook(command)
		if err == nil {
			t.Fatalf("probeHook(%q) must refuse the command rather than run a fragment", command)
		}
		if !strings.Contains(err.Error(), "cannot read the wired command") {
			t.Fatalf("probeHook(%q) = %v, want the probe's own verdict", command, err)
		}
	}
}

// Reading the command with the shared rule must not change the command that gets executed: the probe has
// to keep running the double-quoted path sync writes, and keep reporting a wiring that exits non-zero.
func TestProbeHookRunsTheWiredCommand(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Program Files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := probeHook(`"` + write("gate", "#!/bin/sh\nexit 0\n") + `"`); err != nil {
		t.Fatalf("a quoted path with a space that answers must pass the probe: %v", err)
	}
	if err := probeHook(`"` + write("broken", "#!/bin/sh\nexit 3\n") + `"`); err == nil {
		t.Fatal("a wired command that exits non-zero must fail the probe")
	}
}

const cliFindingPlan = `## Findings

| Id | Finding | Severity | Data safe? | Evidence id | Pinning test | Status | Verdict by / date | Reason | Fingerprint |
|---|---|---|---|---|---|---|---|---|---|

## Evidence ledger

| Id | Claim |
|---|---|
| E1 | observed |
`

func cliFindingArgs(path, id, evidence string, status ...string) []string {
	findingStatus := "open"
	if len(status) > 0 {
		findingStatus = status[0]
	}
	return []string{
		"plan", "add-finding", "--path", path,
		"--id", id, "--location", "src/a.go:1", "--severity", "bug",
		"--data-safe", "yes", "--evidence", evidence, "--test", "",
		"--status", findingStatus, "--verdict-by", "me / 2026-09-10", "--reason", "not pinned",
		"--fingerprint", "",
	}
}

func TestPlanAddFindingCLI(t *testing.T) {
	bin := buildCLI(t)
	path := filepath.Join(t.TempDir(), "test-plan.md")
	if err := os.WriteFile(path, []byte(cliFindingPlan), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, cliFindingArgs(path, "F1", "E1")...)
	if code != 0 || !strings.Contains(out, "added F1") {
		t.Fatalf("successful insertion = %d %q", code, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "| F1 | src/a.go:1 | bug | yes | E1 |  | open | me / 2026-09-10 | not pinned | - |") {
		t.Fatalf("inserted row missing:\n%s", raw)
	}
}

func TestPlanAddFindingUsageRefusalExits2(t *testing.T) {
	bin := buildCLI(t)
	path := filepath.Join(t.TempDir(), "not-created.md")
	out, code := runCLI(t, bin, cliFindingArgs(path, "", "E1")...)
	if code != 2 || !strings.Contains(out, "--id") {
		t.Fatalf("missing required value = %d %q", code, out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("usage refusal must not create the plan: %v", err)
	}
}

func TestPlanAddFindingValueRefusalExits2(t *testing.T) {
	bin := buildCLI(t)
	path := filepath.Join(t.TempDir(), "test-plan.md")
	if err := os.WriteFile(path, []byte(cliFindingPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, cliFindingArgs(path, "F1", "E1", "resolved")...)
	if code != 2 || !strings.Contains(out, "open, confirmed, fixed, rejected, wontfix") {
		t.Fatalf("invalid status = %d %q", code, out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("value refusal must leave the plan byte-identical")
	}
}

func TestPlanAddFindingPlanRefusalExits1(t *testing.T) {
	bin := buildCLI(t)
	path := filepath.Join(t.TempDir(), "test-plan.md")
	plan := strings.Replace(cliFindingPlan, "| E1 | observed |\n", "", 1)
	if err := os.WriteFile(path, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	out, code := runCLI(t, bin, cliFindingArgs(path, "F1", "E1")...)
	if code != 1 || !strings.Contains(out, "not a row in the Evidence ledger") {
		t.Fatalf("plan refusal = %d %q", code, out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("plan refusal must leave the plan byte-identical")
	}
}

func TestPlanAddFindingHelp(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "plan", "add-finding", "--help")
	if code != 2 {
		t.Fatalf("help exit = %d, want 2\n%s", code, out)
	}
	for _, want := range []string{"-id", "-location", "-data-safe", "-fingerprint"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

// A docker binary that is not there is not a failing row: nothing starts, so no container ever exists and the
// refusal has to name the sandbox rather than the command. exec reports a binary it could not find or start as
// an *exec.Error, while a docker that ran and failed reports an *exec.ExitError, which is the line this test
// pins. Nothing here starts a container.
func TestSandboxRunnerNamesADockerThatIsNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // an empty directory: there is no docker to find
	run := sandboxRunner(sandboxImageDefault, sandboxReadOnly)
	_, err := run(context.Background(), t.TempDir(), "printf one")
	var refusal evidence.Refusal
	if !errors.As(err, &refusal) || refusal.Reason != evidence.ReasonMisconfigured {
		t.Fatalf("run = %v, want a %s refusal", err, evidence.ReasonMisconfigured)
	}
	if !strings.Contains(refusal.Detail, "docker") {
		t.Fatalf("detail = %q, want it to name the missing docker", refusal.Detail)
	}
}
