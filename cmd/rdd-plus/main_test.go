package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/buildinfo"
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
		{name: "plan with no subcommand exits 2", args: []string{"plan"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "plan with an unknown subcommand exits 2", args: []string{"plan", "bogus"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "plan check on a missing file exits 1", args: []string{"plan", "check", "--path", "/nonexistent/plan.md"}, wantExit: 1, wantOut: "plan check:"},
		{name: "plan gaps on a missing file exits 1", args: []string{"plan", "gaps", "--path", "/nonexistent/plan.md"}, wantExit: 1, wantOut: "plan gaps:"},
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
	for _, sub := range []string{"bench run", "bench score", "bench history", "bench compare", "bench rescore", "plan init", "plan check", "plan gaps"} {
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

func TestShellFields(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`/h/bin/testing-gate gate`, []string{"/h/bin/testing-gate", "gate"}},
		{`"/h/my bin/testing-gate" gate`, []string{"/h/my bin/testing-gate", "gate"}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{`"/h/gate"`, []string{"/h/gate"}},
	}
	for _, tc := range cases {
		got, err := shellFields(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%q -> %q", tc.in, got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%q -> %q, want %q", tc.in, got, tc.want)
			}
		}
	}
	if _, err := shellFields(`"unbalanced`); err == nil {
		t.Fatal("an unbalanced quote must be an error, not a silent split")
	}
}
