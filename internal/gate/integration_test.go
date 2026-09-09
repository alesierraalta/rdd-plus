package gate

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var binaryPath string

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		dir, err := os.MkdirTemp("", "testing-gate-bin-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		binaryPath = filepath.Join(dir, "testing-gate")
		build := exec.Command("go", "build", "-o", binaryPath, "../../cmd/rdd-plus")
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
			os.Exit(1)
		}
		defer os.RemoveAll(dir)
	}
	os.Exit(m.Run())
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test spawns processes")
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRepo returns a repository with one committed source file and one committed test file.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q")
	mustGit(t, dir, "config", "user.email", "t@t")
	mustGit(t, dir, "config", "user.name", "t")
	writeFile(t, filepath.Join(dir, "src", "a.ts"), "export const a = 1;\n")
	writeFile(t, filepath.Join(dir, "tests", "a.test.ts"), "// test\n")
	mustGit(t, dir, "add", "-A")
	mustGit(t, dir, "commit", "-qm", "baseline")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// edit writes a file and pins its mtime: the future for "this session", days ago for "before".
func edit(t *testing.T, dir, rel, content string, when time.Time) {
	t.Helper()
	p := filepath.Join(dir, rel)
	writeFile(t, p, content)
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
}

// transcript writes a JSONL file whose first line carries the session start, one minute ago.
func transcript(t *testing.T, extra ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := []string{`{"type":"x","timestamp":"` + time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano) + `"}`}
	lines = append(lines, extra...)
	writeFile(t, p, strings.Join(lines, "\n")+"\n")
	return p
}

type hookRun struct {
	code    int
	out     string
	err     string
	entries []map[string]any
}

func runBinary(t *testing.T, bin, cwd string, stdin string, extraEnv ...string) hookRun {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "gate.jsonl")
	cmd := exec.Command(bin, "gate")
	if strings.HasSuffix(bin, ".mjs") {
		cmd = exec.Command("node", bin)
	}
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "TESTING_GATE_LOG="+logFile)
	cmd.Env = append(cmd.Env, extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	r := hookRun{code: code, out: stdout.String(), err: stderr.String()}
	if data, err := os.ReadFile(logFile); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var e map[string]any
			if json.Unmarshal([]byte(line), &e) == nil {
				r.entries = append(r.entries, e)
			}
		}
	}
	return r
}

func payload(cwd, transcriptPath string, more map[string]any) string {
	m := map[string]any{"cwd": cwd, "transcript_path": transcriptPath}
	for k, v := range more {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// reason returns the additionalContext when the hook fired, or "" and false when silent.
func reason(t *testing.T, r hookRun) (string, bool) {
	t.Helper()
	if strings.TrimSpace(r.out) == "" {
		return "", false
	}
	var j struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.out), &j); err != nil {
		t.Fatalf("stdout is not a JSON object: %q", r.out)
	}
	if j.HookSpecificOutput.HookEventName != "Stop" {
		t.Fatalf("hookEventName=%q", j.HookSpecificOutput.HookEventName)
	}
	return j.HookSpecificOutput.AdditionalContext, true
}

func namedFiles(ctx string) []string {
	files := []string{}
	for _, line := range strings.Split(ctx, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "  ...") {
			files = append(files, strings.TrimSpace(line))
		}
	}
	sort.Strings(files)
	return files
}

func lastEntry(r hookRun) map[string]any {
	if len(r.entries) == 0 {
		return nil
	}
	return r.entries[len(r.entries)-1]
}

// scenario is one class of the contract, reusable by the Go suite and the differential test.
type scenario struct {
	name      string
	setup     func(t *testing.T) (cwd string, stdin string)
	wantFire  bool
	wantFiles []string
	check     func(t *testing.T, r hookRun)
}

func fresh() time.Time { return time.Now().Add(120 * time.Second) }
func stale() time.Time { return time.Now().Add(-72 * time.Hour) }

func repoScenarios() []scenario {
	return []scenario{
		{name: "tracked source modified during the session fires and is named", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/a.ts"}},
		{name: "source modified before the session is silent", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", stale())
			return d, payload(d, tr, nil)
		}},
		{name: "test-only change is silent", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "tests/a.test.ts", "// changed\n", fresh())
			return d, payload(d, tr, nil)
		}},
		{name: "docs-only change is silent", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "README.md", "# x\n", fresh())
			return d, payload(d, tr, nil)
		}},
		{name: "renamed source counts under its new name", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			mustGit(t, d, "mv", "src/a.ts", "src/c.ts")
			now := fresh()
			if err := os.Chtimes(filepath.Join(d, "src", "c.ts"), now, now); err != nil {
				t.Fatal(err)
			}
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/c.ts"}},
		{name: "a path with a space counts", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/with space.ts", "export const s = 1;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/with space.ts"}},
		{name: "a new untracked directory is expanded to its files", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/newdir/deep.ts", "export const q = 1;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/newdir/deep.ts"}},
		{name: "a test tree is excluded while a lookalike directory counts", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/__tests__/x.ts", "x", fresh())
			edit(t, d, "src/contest/y.ts", "export const y = 1;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/contest/y.ts"}},
		{name: "a listing mention of the skill does not silence the gate", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t, "- test-strategy: Trigger: haz el testing ... exploit-testing ...")
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/a.ts"}},
		{name: "a real Skill call silences the gate", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t, `{"type":"tool_use","name":"Skill","input":{"skill":"test-strategy"}}`)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, map[string]any{"session_id": "skillcall"})
		}, check: func(t *testing.T, r hookRun) {
			e := lastEntry(r)
			if e == nil || e["fired"] != false || !reflect.DeepEqual(e["skills_loaded"], []any{"test-strategy"}) {
				t.Fatalf("unexpected log entry: %v", e)
			}
		}},
		{name: "a SKILL.md read of exploit-testing silences the gate", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t, `{"input":{"file_path":"/home/x/.claude/skills/exploit-testing/SKILL.md"}}`)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, nil)
		}},
		{name: "a non-adversarial sibling alone does not silence the gate", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t, `{"input":{"file_path":"/home/x/.claude/skills/no-excess-tests/SKILL.md"}}`)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/a.ts"}},
		{name: "missing transcript is treated as no skill loaded", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, filepath.Join(t.TempDir(), "missing.jsonl"), nil)
		}, wantFire: true, wantFiles: []string{"src/a.ts"}},
		{name: "opt-out file at the root silences the repository", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			writeFile(t, filepath.Join(d, ".no-testing-gate"), "")
			return d, payload(d, tr, nil)
		}},
		{name: "opt-out at the root also covers a cwd in a subdirectory", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			writeFile(t, filepath.Join(d, ".no-testing-gate"), "")
			return d, payload(filepath.Join(d, "src"), tr, nil)
		}},
		{name: "a fresh repository with no commits and a new source directory fires", setup: func(t *testing.T) (string, string) {
			d := t.TempDir()
			mustGit(t, d, "init", "-q")
			tr := transcript(t)
			edit(t, d, "src/n.ts", "export const n = 1;\n", fresh())
			return d, payload(d, tr, nil)
		}, wantFire: true, wantFiles: []string{"src/n.ts"}},
		{name: "stop_hook_active is silent even with changes", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			tr := transcript(t)
			edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
			return d, payload(d, tr, map[string]any{"stop_hook_active": true})
		}, check: func(t *testing.T, r hookRun) {
			if len(r.entries) != 0 {
				t.Fatalf("loop guard must not log, got %v", r.entries)
			}
		}},
		{name: "nothing changed is silent and logs zero files", setup: func(t *testing.T) (string, string) {
			d := newRepo(t)
			return d, payload(d, transcript(t), map[string]any{"session_id": "clean"})
		}, check: func(t *testing.T, r hookRun) {
			e := lastEntry(r)
			if e == nil || e["changed_source"] != float64(0) {
				t.Fatalf("expected a zero-file entry, got %v", e)
			}
		}},
	}
}

func assertScenario(t *testing.T, sc scenario, r hookRun) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.err)
	}
	if r.err != "" {
		t.Fatalf("stderr must be empty, got %q", r.err)
	}
	ctx, fired := reason(t, r)
	if fired != sc.wantFire {
		t.Fatalf("fired=%v, want %v (stdout %q, entry %v)", fired, sc.wantFire, r.out, lastEntry(r))
	}
	if sc.wantFire {
		if got := namedFiles(ctx); !reflect.DeepEqual(got, sc.wantFiles) {
			t.Fatalf("named files %q, want %q", got, sc.wantFiles)
		}
		if strings.Count(strings.TrimSpace(r.out), "\n") != 0 {
			t.Fatalf("stdout must be a single line: %q", r.out)
		}
	}
	if sc.check != nil {
		sc.check(t, r)
	}
}

func TestBinaryScenarios(t *testing.T) {
	requireIntegration(t)
	for _, sc := range repoScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			cwd, stdin := sc.setup(t)
			assertScenario(t, sc, runBinary(t, binaryPath, cwd, stdin))
		})
	}
}

func TestBinaryHostileInput(t *testing.T) {
	requireIntegration(t)
	plain := t.TempDir()
	cases := []struct {
		name  string
		cwd   string
		stdin string
		env   []string
	}{
		{"empty stdin", plain, "", nil},
		{"whitespace stdin", plain, "  \n \n", nil},
		{"malformed JSON", plain, "{not json", nil},
		{"array payload", plain, "[1,2]", nil},
		{"null payload", plain, "null", nil},
		{"cwd is not a repository", plain, payload(plain, "/nonexistent", nil), nil},
		{"cwd does not exist", plain, payload(filepath.Join(plain, "nope"), "/nonexistent", nil), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runBinary(t, binaryPath, tc.cwd, tc.stdin, tc.env...)
			if r.code != 0 || r.out != "" || r.err != "" {
				t.Fatalf("code=%d out=%q err=%q", r.code, r.out, r.err)
			}
		})
	}
	t.Run("git missing from PATH: silent, exit 0, no stderr", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		r := runBinary(t, binaryPath, d, payload(d, tr, nil), "PATH=/nonexistent")
		if r.code != 0 || r.out != "" || r.err != "" {
			t.Fatalf("code=%d out=%q err=%q", r.code, r.out, r.err)
		}
	})
}

func TestBinaryFaultsAtTheSeams(t *testing.T) {
	requireIntegration(t)
	t.Run("a huge transcript is scanned inside the hook budget", func(t *testing.T) {
		d := newRepo(t)
		lines := make([]string, 0, 20001)
		for i := 0; i < 20000; i++ {
			lines = append(lines, `{"i":`+fmt.Sprint(i)+`,"text":"`+strings.Repeat("x", 200)+`"}`)
		}
		lines = append(lines, `{"input":{"file_path":"/home/x/.claude/skills/test-strategy/SKILL.md"}}`)
		tr := transcript(t, lines...)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		started := time.Now()
		r := runBinary(t, binaryPath, d, payload(d, tr, nil))
		if r.out != "" {
			t.Fatalf("the skill on the last line must silence the gate")
		}
		if time.Since(started) > 5*time.Second {
			t.Fatalf("took %v", time.Since(started))
		}
	})
	t.Run("a flood of entries marks the checkout as not a project", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		junk := filepath.Join(d, "junk")
		if err := os.MkdirAll(junk, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i <= MaxStatusEntries; i++ {
			if err := os.WriteFile(filepath.Join(junk, fmt.Sprintf("f%d.txt", i)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		r := runBinary(t, binaryPath, d, payload(d, tr, map[string]any{"session_id": "flood"}))
		if r.out != "" {
			t.Fatalf("a home-directory-sized checkout must not fire")
		}
		if e := lastEntry(r); e == nil || e["skipped"] != "too_many_entries" {
			t.Fatalf("expected skipped=too_many_entries, got %v", e)
		}
	})
}

func TestBinaryProperties(t *testing.T) {
	requireIntegration(t)
	t.Run("idempotent: two runs on the same state agree", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		a := runBinary(t, binaryPath, d, payload(d, tr, nil)).out
		b := runBinary(t, binaryPath, d, payload(d, tr, nil)).out
		if a != b {
			t.Fatalf("decisions differ:\n%s\n%s", a, b)
		}
	})
	t.Run("metamorphic: a non-source file does not change the decision", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		before, _ := reason(t, runBinary(t, binaryPath, d, payload(d, tr, nil)))
		edit(t, d, "notes.md", "# n\n", fresh())
		after, _ := reason(t, runBinary(t, binaryPath, d, payload(d, tr, nil)))
		if before != after {
			t.Fatalf("decision changed")
		}
	})
	t.Run("metamorphic: the opt-out flips fire to silent and back", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		if _, on := reason(t, runBinary(t, binaryPath, d, payload(d, tr, nil))); !on {
			t.Fatal("expected fire")
		}
		flag := filepath.Join(d, ".no-testing-gate")
		writeFile(t, flag, "")
		if _, on := reason(t, runBinary(t, binaryPath, d, payload(d, tr, nil))); on {
			t.Fatal("expected silence with the opt-out")
		}
		if err := os.Remove(flag); err != nil {
			t.Fatal(err)
		}
		if _, on := reason(t, runBinary(t, binaryPath, d, payload(d, tr, nil))); !on {
			t.Fatal("expected fire again")
		}
	})
	t.Run("log line carries the decision fields", func(t *testing.T) {
		d := newRepo(t)
		tr := transcript(t)
		edit(t, d, "src/a.ts", "export const a = 2;\n", fresh())
		r := runBinary(t, binaryPath, d, payload(d, tr, map[string]any{"session_id": "s-1"}))
		e := lastEntry(r)
		if e == nil {
			t.Fatal("no log entry")
		}
		if e["session"] != "s-1" || e["repo"] != filepath.Base(d) || e["changed_source"] != float64(1) || e["fired"] != true {
			t.Fatalf("unexpected entry %v", e)
		}
		if _, ok := e["ts"].(string); !ok {
			t.Fatalf("ts missing: %v", e)
		}
		if !reflect.DeepEqual(e["skills_loaded"], []any{}) {
			t.Fatalf("skills_loaded must be an empty array, got %v", e["skills_loaded"])
		}
	})
}

// TestDifferentialAgainstNode runs the Node hook and the Go binary on identical inputs and
// requires the same decision and the same named files. The Node hook bounds the session by the
// transcript's birth time, so every scenario creates the transcript before editing.
func TestDifferentialAgainstNode(t *testing.T) {
	requireIntegration(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	nodeHook := filepath.Join(home, ".claude", "hooks", "testing-gate.mjs")
	if _, err := os.Stat(nodeHook); err != nil {
		t.Skip("Node hook not present")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	for _, sc := range repoScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			cwd, stdin := sc.setup(t)
			goRun := runBinary(t, binaryPath, cwd, stdin)
			nodeRun := runBinary(t, nodeHook, cwd, stdin)
			goCtx, goFired := reason(t, goRun)
			nodeCtx, nodeFired := reason(t, nodeRun)
			if goFired != nodeFired {
				t.Fatalf("decision differs: go=%v node=%v (go entry %v, node entry %v)", goFired, nodeFired, lastEntry(goRun), lastEntry(nodeRun))
			}
			if goFired && !reflect.DeepEqual(namedFiles(goCtx), namedFiles(nodeCtx)) {
				t.Fatalf("named files differ: go=%q node=%q", namedFiles(goCtx), namedFiles(nodeCtx))
			}
		})
	}
}
