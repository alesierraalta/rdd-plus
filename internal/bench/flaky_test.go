package bench

import (
	"os/exec"
	"testing"
	"time"
)

// A defect that only manifests when two goroutines interleave is not observed on every run, so
// one attempt is not enough to say a test cannot distinguish it.
func TestAttemptsForDefect(t *testing.T) {
	cases := map[string]int{
		"data-race":     3,
		"race":          3,
		"concurrency":   3,
		"timing":        3,
		"off-by-one":    1,
		"normalization": 1,
		"":              1,
	}
	for class, want := range cases {
		if got := attemptsForDefect(Defect{Class: class}); got != want {
			t.Errorf("class %q: %d attempts, want %d", class, got, want)
		}
	}
}

// Repeating a deterministic case must not change its verdict.
func TestRepeatedAttemptsDoNotChangeADeterministicVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("runs node")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	caseDir, key := nodeCase(t)
	key.Defects[0].Class = "data-race" // forces the retry path on a deterministic fixture
	ws := agentWorkspace(t, caseDir, map[string]string{
		"tests/agent.test.js": "const test = require('node:test'); const assert = require('node:assert'); const s = require('../src.js');\n" +
			"test('catches d1', () => { assert.equal(s.d1(), 'ok'); });\n",
	})
	got := Discriminate(caseDir, ws, key, 2*time.Minute)
	if !got.Caught["D1"] || got.Caught["D2"] {
		t.Fatalf("caught = %v, notes %v", got.Caught, got.Notes)
	}
	if got.Attempts["D1"] != 3 || got.Attempts["D2"] != 1 {
		t.Fatalf("attempts = %v", got.Attempts)
	}
}
