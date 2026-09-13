package bench

import (
	"os"
	"path/filepath"
	"strings"
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
	argv, parser, err := perTestCommand(suite)
	if err != nil {
		t.Fatalf("a well-formed suite reports no split error: %v", err)
	}
	if len(argv) != 2 || argv[0] != "sh" || argv[1] != script {
		t.Fatalf("a quoted path with a space must be one word: %q", argv)
	}
	if parser != nil {
		t.Fatal("a sh suite has no per-test format, got one")
	}
	res, err := RunSuite(dir, suite, time.Minute)
	if err != nil {
		t.Fatalf("suite %q: %v", suite, err)
	}
	if res.ExitCode != 0 || !res.Green {
		t.Fatalf("suite %q: exit %d, green %v, output %q", suite, res.ExitCode, res.Green, res.Output)
	}

	// The per-test rewrite inserts its reporter flag into the split words without re-splitting what it
	// kept, so the quoted argument survives the rewrite too.
	argv, parser, err = perTestCommand(`node --test "` + suiteDir + `/x.test.js"`)
	if err != nil {
		t.Fatalf("a well-formed suite reports no split error: %v", err)
	}
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

// A suite whose quote is left open is not a command anyone can read, so the splitter reports it. Running the
// words read so far would run a command nobody wrote: the quote is dropped, not preserved, so `sh -c "touch
// /tmp/x/marker` would run `touch /tmp/x/marker` and report a suite failure the fixture never had. The
// marker proves the refusal is pre-execution: nothing ran, so the file was never created.
func TestSuiteWithAnUnterminatedQuoteIsRefusedBeforeItRuns(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	suite := `sh -c "touch ` + marker

	res, err := RunSuite(dir, suite, time.Minute)
	if err == nil {
		t.Fatalf("a suite the splitter cannot read must be refused: %+v", res)
	}
	if !strings.Contains(err.Error(), "unterminated quote") {
		t.Fatalf("the refusal must be the shared splitter's error, not a new one: %v", err)
	}
	if res.Command != suite || res.ExitCode != -1 || res.Green {
		t.Fatalf("a refused suite keeps its command and records no outcome: %+v", res)
	}
	if _, _, err := suiteOn(dir, "", dir, nil, suite, time.Minute); err == nil {
		t.Fatal("the whole-suite check must report the suite it cannot read")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the malformed suite ran: the dropped quote turned it into a touch command")
	}
}

// The per-test path reads the same field with the same rule, so it must refuse the same malformed suite
// before it stages or runs anything. The marker is what makes this behavioral rather than an assertion about
// an error value: had the words read so far been run, the file would exist.
func TestPerTestSuiteWithAnUnterminatedQuoteIsRefusedBeforeItRuns(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	suite := `sh -c "touch ` + marker

	if _, _, err := perTestCommand(suite); err == nil {
		t.Fatal("the per-test rewrite must report a suite it cannot read")
	}
	outcomes, code, out, err := perTest(dir, suite, time.Minute)
	if err == nil || outcomes != nil || code != -1 || out != "" {
		t.Fatalf("a refused per-test run records no outcome: %v %v %q %v", outcomes, code, out, err)
	}
	_, _, _, terr := testsOn(t.TempDir(), "", dir, nil, suite, time.Minute)
	if terr == nil {
		t.Fatal("the per-test check must report the suite it cannot read")
	}
	if !strings.Contains(terr.Error(), "unterminated quote") {
		t.Fatalf("the refusal must be the shared splitter's error, not a new one: %v", terr)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the malformed suite ran: the dropped quote turned it into a touch command")
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
		argv, _, err := perTestCommand(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
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
