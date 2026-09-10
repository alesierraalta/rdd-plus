package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTAP(t *testing.T) {
	out := "TAP version 13\n# Subtest: escapes tags\nok 1 - escapes tags\n  ---\n  duration_ms: 0.8\n  ...\nnot ok 2 - escapes single quotes\n  ---\n  error: 'x'\n  ...\n# Subtest: file\n    ok 1 - nested case\nnot ok 3 - file\n1..3\n"
	got := parseTAP(out)
	want := map[string]bool{"escapes tags": true, "escapes single quotes": false, "file/nested case": true, "file": false}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if p, ok := got[k]; !ok || p != v {
			t.Fatalf("%s: got %v/%v, want %v (%v)", k, p, ok, v, got)
		}
	}
}

func TestParseGoJSON(t *testing.T) {
	out := `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0}
{"Action":"fail","Package":"p","Test":"TestB","Elapsed":0}
{"Action":"pass","Package":"p/sub","Test":"TestA/case_1","Elapsed":0}
{"Action":"fail","Package":"p","Elapsed":0.1}
`
	got := parseGoJSON(out)
	want := map[string]bool{"p.TestA": true, "p.TestB": false, "p/sub.TestA/case_1": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %v want %v", k, got[k], v)
		}
	}
}

func TestPerTestCommand(t *testing.T) {
	cases := map[string][]string{
		"node --test":         {"node", "--test", "--test-reporter=tap"},
		"go test ./...":       {"go", "test", "-json", "./..."},
		"go test -race ./...": {"go", "test", "-json", "-race", "./..."},
		"sh run-tests.sh":     {"sh", "run-tests.sh"},
	}
	for suite, want := range cases {
		got, _ := perTestCommand(suite)
		if len(got) != len(want) {
			t.Fatalf("%q: %v", suite, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q: %v, want %v", suite, got, want)
			}
		}
	}
}

// A real node case: one broken agent test must not hide the test that catches D1.
func TestDiscriminatePerTestSurvivesABrokenTest(t *testing.T) {
	if testing.Short() {
		t.Skip("runs node")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	caseDir, key := nodeCase(t)
	ws := agentWorkspace(t, caseDir, map[string]string{
		"tests/agent.test.js": "const test = require('node:test'); const assert = require('node:assert'); const s = require('../src.js');\n" +
			"test('d1 is fixed', () => { assert.equal(s.d1(), 'ok'); });\n" +
			"test('always broken', () => { assert.equal(1, 2); });\n",
	})
	got := Discriminate(caseDir, ws, key, 2*time.Minute)
	if !got.Checked || got.AllGreen {
		t.Fatalf("checked=%v allGreen=%v notes=%v", got.Checked, got.AllGreen, got.Notes)
	}
	if !got.Caught["D1"] || got.Caught["D2"] {
		t.Fatalf("caught = %v notes=%v", got.Caught, got.Notes)
	}
	if got.BrokenTests != 1 {
		t.Fatalf("broken tests = %d, want 1", got.BrokenTests)
	}
	if by := got.CaughtBy["D1"]; len(by) != 1 || by[0] != "d1 is fixed" {
		t.Fatalf("caught by = %v", got.CaughtBy)
	}
}

// A test that pins the defective behaviour is not a catch: it is green while the defect is there
// and red once it is fixed, so it resists the fix instead of demanding it.
func TestDiscriminateReportsInvertedTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs node")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	caseDir, key := nodeCase(t)
	ws := agentWorkspace(t, caseDir, map[string]string{
		"tests/agent.test.js": "const test = require('node:test'); const assert = require('node:assert'); const s = require('../src.js');\n" +
			"test('pins the bug', () => { assert.equal(s.d1(), 'bug'); });\n" +
			"test('catches d2', () => { assert.equal(s.d2(), 'ok'); });\n" +
			"test('red everywhere', () => { assert.equal(s.d1(), 'neither'); });\n",
	})
	got := Discriminate(caseDir, ws, key, 2*time.Minute)
	if got.Caught["D1"] {
		t.Fatal("a test asserting the defective value must not count as catching it")
	}
	if !got.Caught["D2"] {
		t.Fatalf("the sound test must still count: %v", got.Notes)
	}
	if got.InvertedTests != 1 {
		t.Fatalf("inverted = %d, want 1 (%v)", got.InvertedTests, got.Notes)
	}
	if got.BrokenTests != 2 {
		t.Fatalf("broken = %d, want 2 (the inverted one and the one red everywhere)", got.BrokenTests)
	}
	if by := got.InvertedBy["D1"]; len(by) != 1 || by[0] != "pins the bug" {
		t.Fatalf("inverted by = %v", got.InvertedBy)
	}
	if by := got.InvertedBy["D2"]; len(by) != 0 {
		t.Fatalf("a test red everywhere is broken, not inverted: %v", got.InvertedBy)
	}
	if !strings.Contains(strings.Join(got.Notes, " "), "pin the defective behaviour") {
		t.Fatalf("notes must name the failure: %v", got.Notes)
	}
}

// nodeCase is a two-defect fixture whose suite reports one result per test.
func nodeCase(t *testing.T) (string, Key) {
	t.Helper()
	caseDir := t.TempDir()
	files := map[string]string{
		"fixture/src.js":              "module.exports = { d1: () => 'bug', d2: () => 'bug' };\n",
		"fixture/tests/happy.test.js": "const test = require('node:test'); const assert = require('node:assert'); const s = require('../src.js');\ntest('exports', () => { assert.equal(typeof s.d1, 'function'); });\n",
		"fix/all/src.js":              "module.exports = { d1: () => 'ok', d2: () => 'ok' };\n",
		"fix/keep-D1/src.js":          "module.exports = { d1: () => 'bug', d2: () => 'ok' };\n",
		"fix/keep-D2/src.js":          "module.exports = { d1: () => 'ok', d2: () => 'bug' };\n",
	}
	for p, c := range files {
		full := filepath.Join(caseDir, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return caseDir, Key{ID: "node", Suite: "node --test", Defects: []Defect{{ID: "D1"}, {ID: "D2"}}}
}
