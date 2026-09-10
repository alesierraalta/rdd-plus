package bench

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// FixDir holds the fixed versions of a case: fix/all with every defect fixed, and fix/keep-<ID>
// with every defect but <ID> fixed. Each holds only the files that differ from the fixture.
const FixDir = "fix"

// CatchResult says which planted defects the agent's tests actually distinguish. A defect is
// caught when the agent's test files are green on the fully fixed code and red on the code where
// only that defect remains; reporting the defect in the plan is a separate measure.
type CatchResult struct {
	Checked     bool                `json:"checked"`              // fix/all exists for the case
	TestFiles   []string            `json:"test_files,omitempty"` // agent test files carried into the check
	AllGreen    bool                `json:"all_green"`            // every test passes on the fully fixed code
	BrokenTests int                 `json:"broken_tests"`         // tests red on the fully fixed code; they prove nothing
	Caught      map[string]bool     `json:"caught"`               // defect id -> caught
	CaughtBy    map[string][]string `json:"caught_by,omitempty"`  // defect id -> tests green on fix/all and red on keep-<id>
	Notes       []string            `json:"notes,omitempty"`
}

// Count returns how many defects were caught.
func (c CatchResult) Count() int {
	n := 0
	for _, v := range c.Caught {
		if v {
			n++
		}
	}
	return n
}

var (
	testPathRe    = regexp.MustCompile(`(^|/)(tests?|__tests__|spec)/|(_test\.go|\.(test|spec)\.[cm]?[jt]sx?|_spec\.rb|(^|/)test_[^/]*\.py)$`)
	ignoredPathRe = regexp.MustCompile(`(^|/)(\.git|node_modules|vendor|\.atl|\.claude|docs|testdata)(/|$)`)
)

// isTestFile recognises the test files of the suites the benchmark runs.
func isTestFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	return !ignoredPathRe.MatchString(rel) && testPathRe.MatchString(rel)
}

// Discriminate runs the check for one agent workspace against the case's fixed versions.
func Discriminate(caseDir, ws string, key Key, timeout time.Duration) CatchResult {
	res := CatchResult{Caught: map[string]bool{}}
	fixture := filepath.Join(caseDir, FixtureDir)
	all := filepath.Join(caseDir, FixDir, "all")
	if st, err := os.Stat(all); err != nil || !st.IsDir() {
		res.Notes = append(res.Notes, "no fix/all directory; check skipped")
		return res
	}
	res.Checked = true
	tests, err := agentTestFiles(fixture, ws)
	if err != nil {
		res.Notes = append(res.Notes, "listing agent tests: "+err.Error())
		return res
	}
	res.TestFiles = tests
	if len(tests) == 0 {
		res.Notes = append(res.Notes, "the agent added or changed no test file")
		return res
	}
	onAll, code, out, err := testsOn(fixture, all, ws, tests, key.Suite, timeout)
	if err != nil {
		res.Notes = append(res.Notes, "fix/all: "+err.Error())
		return res
	}
	res.AllGreen = code == 0
	var trusted []string // tests green on the correct code; only these can catch anything
	for name, passed := range onAll {
		if passed {
			trusted = append(trusted, name)
		} else {
			res.BrokenTests++
		}
	}
	sort.Strings(trusted)
	if len(trusted) == 0 {
		// A test red on correct code is broken or written to a different API: it can prove nothing.
		res.Notes = append(res.Notes, "no agent test is green on the fully fixed code: "+tail(out, 400))
		return res
	}
	if res.BrokenTests > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("%d test(s) red on the fully fixed code were ignored", res.BrokenTests))
	}
	for _, d := range key.Defects {
		overlay := filepath.Join(caseDir, FixDir, "keep-"+d.ID)
		if len(key.Defects) == 1 {
			overlay = "" // the pristine fixture is the code where the only defect remains
		} else if st, err := os.Stat(overlay); err != nil || !st.IsDir() {
			res.Notes = append(res.Notes, fmt.Sprintf("%s: no fix/keep-%s directory; not checkable", d.ID, d.ID))
			continue
		}
		onKeep, _, _, err := testsOn(fixture, overlay, ws, tests, key.Suite, timeout)
		if err != nil {
			res.Notes = append(res.Notes, d.ID+": "+err.Error())
			continue
		}
		for _, name := range trusted {
			if passed, ran := onKeep[name]; ran && !passed {
				if res.CaughtBy == nil {
					res.CaughtBy = map[string][]string{}
				}
				res.CaughtBy[d.ID] = append(res.CaughtBy[d.ID], name)
			}
		}
		res.Caught[d.ID] = len(res.CaughtBy[d.ID]) > 0
	}
	return res
}

// agentTestFiles lists test files the agent added or changed, relative to the workspace.
func agentTestFiles(fixture, ws string) ([]string, error) {
	var out []string
	err := filepath.Walk(ws, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(ws, path)
		if info.IsDir() {
			if rel != "." && ignoredPathRe.MatchString(filepath.ToSlash(rel)+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTestFile(rel) {
			return nil
		}
		got, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if orig, err := os.ReadFile(filepath.Join(fixture, rel)); err == nil && bytes.Equal(orig, got) {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

// suiteOn runs the suite on fixture + overlay + the agent's test files, in a fresh directory.
func suiteOn(fixture, overlay, ws string, tests []string, suite string, timeout time.Duration) (green bool, output string, err error) {
	dir, err := stage(fixture, overlay, ws, tests)
	if err != nil {
		return false, "", err
	}
	defer os.RemoveAll(dir)
	r := RunSuite(dir, suite, timeout)
	return r.ExitCode == 0, r.Output, nil
}

// testsOn is suiteOn with one outcome per test.
func testsOn(fixture, overlay, ws string, tests []string, suite string, timeout time.Duration) (map[string]bool, int, string, error) {
	dir, err := stage(fixture, overlay, ws, tests)
	if err != nil {
		return nil, -1, "", err
	}
	defer os.RemoveAll(dir)
	outcomes, code, out := perTest(dir, suite, timeout)
	return outcomes, code, out, nil
}

// stage builds fixture + overlay + the agent's test files in a fresh directory.
func stage(fixture, overlay, ws string, tests []string) (string, error) {
	dir, err := os.MkdirTemp("", "rdd-plus-catch-")
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) { os.RemoveAll(dir); return "", err }
	if err := copyTree(fixture, dir); err != nil {
		return fail(err)
	}
	if overlay != "" {
		if err := copyTree(overlay, dir); err != nil {
			return fail(err)
		}
	}
	for _, rel := range tests {
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fail(err)
		}
		data, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fail(err)
		}
	}
	return dir, nil
}
