package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A suite command is one field stored as one string, so it is split with the same quote-aware rule the
// hook command is: the two used to be split by different rules, which is the drift that produced a
// misread hook. A suite whose path contains a space is one word. Splitting on whitespace alone runs the
// truncated head of that path, and the run records a suite failure the fixture never had.
func TestSuiteCommandIsSplitWithTheSharedQuoteRule(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "my suite")
	if err := os.MkdirAll(suiteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(suiteDir, "run-tests.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	suite := `sh "` + script + `"`
	argv, parser := perTestCommand(suite)
	if len(argv) != 2 || argv[0] != "sh" || argv[1] != script {
		t.Fatalf("a quoted path with a space must be one word: %q", argv)
	}
	if parser != nil {
		t.Fatal("a sh suite has no per-test format, got one")
	}
	if res := RunSuite(dir, suite, time.Minute); res.ExitCode != 0 || !res.Green {
		t.Fatalf("suite %q: exit %d, green %v, output %q", suite, res.ExitCode, res.Green, res.Output)
	}

	// The per-test rewrite inserts its reporter flag into the split words without re-splitting what it
	// kept, so the quoted argument survives the rewrite too.
	argv, parser = perTestCommand(`node --test "` + suiteDir + `/x.test.js"`)
	want := []string{"node", "--test", "--test-reporter=tap", filepath.Join(suiteDir, "x.test.js")}
	if parser == nil {
		t.Fatalf("node --test must keep its per-test format: %q", argv)
	}
	if len(argv) != len(want) {
		t.Fatalf("node rewrite: %q, want %q", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("node rewrite: %q, want %q", argv, want)
		}
	}
}

// No suite in today's corpus carries a quote, so the shared rule must leave those commands word-for-word
// identical: a suite split differently would change what a reading measured. The per-test rewrite still
// injects its reporter flag right after the runner and its subcommand.
func TestSuiteCommandWithoutQuotesIsUnchanged(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`node --test`, []string{"node", "--test", "--test-reporter=tap"}},
		{`go test -race ./...`, []string{"go", "test", "-json", "-race", "./..."}},
		{`go test ./...`, []string{"go", "test", "-json", "./..."}},
		{`sh run-tests.sh`, []string{"sh", "run-tests.sh"}},
		{"  spaced   out  ", []string{"spaced", "out"}},
		{"a\tb\tc", []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		argv, _ := perTestCommand(tc.in)
		if len(argv) != len(tc.want) {
			t.Fatalf("%q -> %q, want %q", tc.in, argv, tc.want)
		}
		for i := range argv {
			if argv[i] != tc.want[i] {
				t.Fatalf("%q -> %q, want %q", tc.in, argv, tc.want)
			}
		}
	}
}
