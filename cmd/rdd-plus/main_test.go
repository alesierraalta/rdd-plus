package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		{name: "version prints the version and exits 0", args: []string{"version"}, wantExit: 0, wantOut: "dev"},
		{name: "bench with no subcommand exits 2", args: []string{"bench"}, wantExit: 2, wantOut: "usage: rdd-plus"},
		{name: "bench with an unknown subcommand exits 2", args: []string{"bench", "bogus"}, wantExit: 2, wantOut: "usage: rdd-plus"},
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

// A usage text that does not list a command it accepts sends users to the wrong place.
func TestUsageListsEveryBenchSubcommand(t *testing.T) {
	for _, sub := range []string{"bench run", "bench score", "bench history", "bench compare", "bench rescore", "plan init", "plan check"} {
		if !strings.Contains(usage, sub) {
			t.Errorf("usage does not document %q", sub)
		}
	}
	for _, cmd := range []string{"gate", "sync", "doctor", "bench", "plan", "version"} {
		if !strings.Contains(usage, "  "+cmd+" ") {
			t.Errorf("usage does not document the %q command", cmd)
		}
	}
}
