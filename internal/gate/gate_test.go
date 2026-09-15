package gate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/plan"
)

type fakeInfo struct {
	name string
	mod  time.Time
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return 0 }
func (f fakeInfo) ModTime() time.Time { return f.mod }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

// fakeRepo stands in for git, the filesystem, and the transcript at the process boundary.
type fakeRepo struct {
	root              string
	status            string
	statusErr         error
	rootErr           error
	files             map[string]time.Time
	optOut            bool
	transcript        string
	transcriptMissing bool
	plan              string
	planPath          string
	planConfig        string
	plans             map[string]string
	planMissing       bool
}

func (f *fakeRepo) deps(now time.Time) Deps {
	return Deps{
		Git: func(dir string, args ...string) (string, error) {
			switch args[0] {
			case "rev-parse":
				if f.rootErr != nil {
					return "", f.rootErr
				}
				return f.root + "\n", nil
			case "status":
				if f.statusErr != nil {
					return "", f.statusErr
				}
				return f.status, nil
			}
			return "", errors.New("unexpected git call")
		},
		Stat: func(p string) (os.FileInfo, error) {
			if p == filepath.Join(f.root, ".no-testing-gate") {
				if f.optOut {
					return fakeInfo{name: ".no-testing-gate"}, nil
				}
				return nil, os.ErrNotExist
			}
			rel, err := filepath.Rel(f.root, p)
			if err != nil {
				return nil, os.ErrNotExist
			}
			if mt, ok := f.files[rel]; ok {
				return fakeInfo{name: filepath.Base(p), mod: mt}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadPlan: func(path string) (string, error) {
			if path == filepath.Join(f.root, plan.ConfigName) {
				if f.planConfig == "" {
					return "", os.ErrNotExist
				}
				return f.planConfig, nil
			}
			rel, err := filepath.Rel(f.root, path)
			if err != nil {
				return "", os.ErrNotExist
			}
			body := f.plan
			if f.plans != nil {
				var ok bool
				body, ok = f.plans[rel]
				if !ok {
					return "", os.ErrNotExist
				}
			} else {
				want := f.planPath
				if want == "" {
					want = plan.DefaultPath
				}
				if rel != want {
					return "", os.ErrNotExist
				}
			}
			if f.planMissing || body == "" {
				return "", os.ErrNotExist
			}
			return body, nil
		},
		OpenTranscript: func(string) (io.ReadCloser, error) {
			if f.transcriptMissing {
				return nil, os.ErrNotExist
			}
			return io.NopCloser(strings.NewReader(f.transcript)), nil
		},
		Now:     now,
		WorkDir: f.root,
	}
}

func porcelain(entries ...string) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e)
		b.WriteByte(0)
	}
	return b.String()
}

func stamped(t time.Time, extra ...string) string {
	lines := []string{`{"type":"x","timestamp":"` + t.UTC().Format(time.RFC3339Nano) + `"}`}
	lines = append(lines, extra...)
	return strings.Join(lines, "\n") + "\n"
}

func TestParsePorcelain(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"rename skips the old path and keeps the new name", porcelain("R  src/c.ts", "src/a.ts"), []string{"src/c.ts"}},
		{"deletion is excluded", porcelain(" D src/b.ts", " M src/a.ts"), []string{"src/a.ts"}},
		{"short fields are ignored", porcelain("??", " M src/a.ts", "x"), []string{"src/a.ts"}},
		{"paths with spaces are raw, not quoted", porcelain("?? src/with space.ts"), []string{"src/with space.ts"}},
		{"trailing NUL yields no phantom entry", porcelain(" M src/a.ts") + "\x00", []string{"src/a.ts"}},
		{"empty status yields nothing", "", []string{}},
		{"copy also skips its source field", porcelain("C  src/d.ts", "src/a.ts", "?? src/e.ts"), []string{"src/d.ts", "src/e.ts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParsePorcelain(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsProductionSource(t *testing.T) {
	positives := []string{"src/a.ts", "lib/x.py", "cmd/main.go", "src/contest/y.ts", "src/with space.ts", "app.mjs"}
	negatives := []string{"tests/a.test.ts", "src/a.spec.js", "src/__tests__/x.ts", "e2e/x.ts", "fixtures/f.ts", "types.d.ts",
		"README.md", ".claude/hooks/x.mjs", "evals/cases/x.js", "node_modules/p/i.js", "vendor/v.go", "dist/b.js", "src/.venv/lib.py"}
	for _, p := range positives {
		t.Run("source "+p, func(t *testing.T) {
			if !IsProductionSource(p) {
				t.Fatalf("%q must count as production source", p)
			}
		})
	}
	for _, p := range negatives {
		t.Run("excluded "+p, func(t *testing.T) {
			if IsProductionSource(p) {
				t.Fatalf("%q must not count as production source", p)
			}
		})
	}
}

func TestSessionStart(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	a := now.Add(-90 * time.Second)
	b := now.Add(-30 * time.Second)
	cases := []struct {
		name  string
		lines string
		want  time.Time
	}{
		{"first timestamped line wins", "garbage\n{\"type\":\"x\"}\n" + stamped(a) + stamped(b), a},
		{"no timestamp falls back to the wide window", "{\"type\":\"x\"}\n{}\n", now.Add(-wideWindow)},
		{"unparseable timestamp is skipped", "{\"timestamp\":\"yesterday\"}\n" + stamped(b), b},
		{"empty input falls back", "", now.Add(-wideWindow)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SessionStart(strings.NewReader(tc.lines), now)
			if !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("a timestamp past the scan cap is not the session start", func(t *testing.T) {
		lines := strings.Repeat("{}\n", maxTimestampScan) + stamped(a)
		if got := SessionStart(strings.NewReader(lines), now); !got.Equal(now.Add(-wideWindow)) {
			t.Fatalf("got %v, want fallback", got)
		}
	})
	t.Run("nil reader falls back", func(t *testing.T) {
		if got := SessionStart(nil, now); !got.Equal(now.Add(-wideWindow)) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestSkillsLoaded(t *testing.T) {
	cases := []struct {
		name  string
		lines string
		want  []string
	}{
		{"listing text does not count", "- test-strategy: Trigger: haz el testing ... exploit-testing ...\n", []string{}},
		{"a Skill tool call counts", `{"type":"tool_use","name":"Skill","input":{"skill":"test-strategy"}}` + "\n", []string{"test-strategy"}},
		{"a SKILL.md read counts", `{"input":{"file_path":"/home/x/.claude/skills/exploit-testing/SKILL.md"}}` + "\n", []string{"exploit-testing"}},
		{"a sibling alone returns only that sibling", `{"input":{"file_path":"/x/.claude/skills/no-excess-tests/SKILL.md"}}` + "\n", []string{"no-excess-tests"}},
		{"order is first seen, no duplicates", `{"skill":"exploit-testing"}` + "\n" + `{"skill":"test-strategy"}` + "\n" + `{"skill":"exploit-testing"}` + "\n", []string{"exploit-testing", "test-strategy"}},
		{"empty transcript", "", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SkillsLoaded(strings.NewReader(tc.lines)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if got := SkillsLoaded(nil); len(got) != 0 || got == nil {
		t.Fatalf("nil reader must yield an empty, non-nil slice")
	}
}

func TestBuildReason(t *testing.T) {
	t.Run("six files shown then the rest counted", func(t *testing.T) {
		files := []string{"a.ts", "b.ts", "c.ts", "d.ts", "e.ts", "f.ts", "g.ts", "h.ts"}
		r := BuildReason(files)
		for _, f := range files[:6] {
			if !strings.Contains(r, "  "+f) {
				t.Fatalf("missing %s", f)
			}
		}
		if strings.Contains(r, "g.ts") || !strings.Contains(r, "... and 2 more") {
			t.Fatalf("overflow not summarised: %q", r)
		}
		if !strings.Contains(r, "changed 8 production source file(s)") {
			t.Fatalf("count missing: %q", r)
		}
	})
	t.Run("one file, no overflow line", func(t *testing.T) {
		r := BuildReason([]string{"src/a.ts"})
		if strings.Contains(r, "more") || !strings.Contains(r, ".no-testing-gate") {
			t.Fatalf("unexpected reason: %q", r)
		}
	})
}

func TestDecide(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	start := now.Add(-60 * time.Second)
	fresh := now.Add(120 * time.Second)
	root := "/repo"
	base := func() *fakeRepo {
		return &fakeRepo{
			root:       root,
			status:     porcelain(" M src/a.ts"),
			files:      map[string]time.Time{"src/a.ts": fresh},
			transcript: stamped(start),
		}
	}
	manyEntries := func(n int) string {
		parts := make([]string, 0, n)
		parts = append(parts, " M src/a.ts")
		for i := 1; i < n; i++ {
			parts = append(parts, fmt.Sprintf("?? junk/f%d.txt", i))
		}
		return porcelain(parts...)
	}

	cases := []struct {
		name    string
		in      Input
		repo    func() *fakeRepo
		fire    bool
		files   []string
		skipped string
		optOut  bool
		loaded  []string
		noEntry bool
	}{
		{name: "loop guard wins over everything", in: Input{StopHookActive: true, TranscriptPath: "t"}, repo: base, noEntry: true},
		{name: "not a repository: silent, no log", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.rootErr = errors.New("not a git repository"); return r }, noEntry: true},
		{name: "git status failure is logged as skipped", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.statusErr = errors.New("boom"); return r }, skipped: "git_status_failed"},
		{name: "too many entries is not a project", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.status = manyEntries(MaxStatusEntries + 1); return r }, skipped: "too_many_entries"},
		{name: "exactly the cap is still a project", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.status = manyEntries(MaxStatusEntries); return r }, fire: true, files: []string{"src/a.ts"}},
		{name: "zero changed files: silent but logged", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.status = ""; return r }, files: []string{}},
		{name: "changed source and no skill: fire naming every file", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo {
			r := base()
			r.status = porcelain(" M src/a.ts", "?? src/b.ts", "?? README.md")
			r.files["src/b.ts"] = fresh
			r.files["README.md"] = fresh
			return r
		}, fire: true, files: []string{"src/a.ts", "src/b.ts"}},
		{name: "adversarial skill loaded: silent with the skill logged", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo {
			r := base()
			r.transcript = stamped(start, `{"name":"Skill","input":{"skill":"test-strategy"}}`)
			return r
		}, files: []string{"src/a.ts"}, loaded: []string{"test-strategy"}},
		{name: "a non-adversarial sibling alone does not silence", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo {
			r := base()
			r.transcript = stamped(start, `{"input":{"file_path":"/x/.claude/skills/no-excess-tests/SKILL.md"}}`)
			return r
		}, fire: true, files: []string{"src/a.ts"}, loaded: []string{"no-excess-tests"}},
		{name: "opt-out at the root silences a cwd in a subdirectory", in: Input{TranscriptPath: "t", Cwd: root + "/sub/dir"}, repo: func() *fakeRepo { r := base(); r.optOut = true; return r }, files: []string{"src/a.ts"}, optOut: true},
		{name: "mtime exactly at the session start counts", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.files["src/a.ts"] = start; return r }, fire: true, files: []string{"src/a.ts"}},
		{name: "mtime one second before the session start does not count", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo { r := base(); r.files["src/a.ts"] = start.Add(-time.Second); return r }, files: []string{}},
		{name: "missing transcript: a file from one hour ago is inside the wide window", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo {
			r := base()
			r.transcriptMissing = true
			r.files["src/a.ts"] = now.Add(-time.Hour)
			return r
		}, fire: true, files: []string{"src/a.ts"}},
		{name: "missing transcript: a file from nine hours ago is outside it", in: Input{TranscriptPath: "t"}, repo: func() *fakeRepo {
			r := base()
			r.transcriptMissing = true
			r.files["src/a.ts"] = now.Add(-9 * time.Hour)
			return r
		}, files: []string{}},
		{name: "empty cwd falls back to the working directory", in: Input{TranscriptPath: "t"}, repo: base, fire: true, files: []string{"src/a.ts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Decide(tc.in, tc.repo().deps(now))
			if res.Fire != tc.fire {
				t.Fatalf("fire=%v, want %v (entry %+v)", res.Fire, tc.fire, res.Entry)
			}
			if tc.noEntry {
				if res.Entry != nil {
					t.Fatalf("expected no log entry, got %+v", res.Entry)
				}
				return
			}
			if res.Entry == nil {
				t.Fatalf("expected a log entry")
			}
			if res.Entry.Skipped != tc.skipped {
				t.Fatalf("skipped=%q, want %q", res.Entry.Skipped, tc.skipped)
			}
			if tc.skipped != "" {
				return
			}
			if !reflect.DeepEqual(res.Files, tc.files) {
				t.Fatalf("files=%q, want %q", res.Files, tc.files)
			}
			if res.Entry.ChangedSource != len(tc.files) {
				t.Fatalf("changed_source=%d, want %d", res.Entry.ChangedSource, len(tc.files))
			}
			if res.Entry.OptedOut != tc.optOut {
				t.Fatalf("opted_out=%v, want %v", res.Entry.OptedOut, tc.optOut)
			}
			wantLoaded := tc.loaded
			if wantLoaded == nil {
				wantLoaded = []string{}
			}
			if !reflect.DeepEqual(res.Entry.SkillsLoaded, wantLoaded) {
				t.Fatalf("skills_loaded=%q, want %q", res.Entry.SkillsLoaded, wantLoaded)
			}
			if res.Entry.Fired != tc.fire {
				t.Fatalf("logged fired=%v, want %v", res.Entry.Fired, tc.fire)
			}
			if tc.fire {
				for _, f := range tc.files {
					if !strings.Contains(res.Reason, "  "+f) {
						t.Fatalf("reason does not name %s: %q", f, res.Reason)
					}
				}
			} else if res.Reason != "" {
				t.Fatalf("silent decision must carry no reason")
			}
		})
	}
}
