package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/assets"
	"github.com/alesierraalta/rdd-plus/internal/sync"
)

func allPresent(name string) (string, error) { return "/usr/bin/" + name, nil }

func synced(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	if _, err := sync.Sync(cfg, "/opt/tools/rdd-plus", sync.Options{}); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestDoctorHealthyAfterSync(t *testing.T) {
	r := Run(synced(t), allPresent)
	if !r.Healthy {
		t.Fatalf("expected healthy, problems: %v", r.Problems)
	}
	if !r.HookWired || !strings.HasSuffix(r.HookCommand, " gate") {
		t.Fatalf("hook not detected: %+v", r)
	}
	for _, s := range r.Skills {
		if !s.Installed || !s.Matches {
			t.Errorf("skill %s: installed=%v matches=%v", s.Name, s.Installed, s.Matches)
		}
	}
	if !strings.Contains(r.String(), "verdict: healthy") {
		t.Fatal("text report should say healthy")
	}
}

func TestDoctorMissingGitIsUnhealthy(t *testing.T) {
	cfg := synced(t)
	noGit := func(name string) (string, error) {
		if name == "git" {
			return "", errors.New("not found")
		}
		return allPresent(name)
	}
	r := Run(cfg, noGit)
	if r.Healthy {
		t.Fatal("missing git must make the report unhealthy")
	}
	found := false
	for _, p := range r.Problems {
		if strings.Contains(p, "git") {
			found = true
		}
	}
	if !found {
		t.Fatalf("problems do not name git: %v", r.Problems)
	}
	if !strings.Contains(r.String(), "ABSENT (required)") {
		t.Fatal("text report should mark git as required and absent")
	}
}

func TestDoctorEmptyConfigDir(t *testing.T) {
	r := Run(t.TempDir(), allPresent)
	if r.Healthy {
		t.Fatal("nothing installed must be unhealthy")
	}
	if r.HookWired {
		t.Fatal("no settings.json, hook cannot be wired")
	}
	if len(r.Problems) < len(assets.SkillNames())+1 {
		t.Fatalf("expected a problem per missing skill plus the hook, got %d", len(r.Problems))
	}
}

func TestDoctorReportsSkillDrift(t *testing.T) {
	cfg := synced(t)
	name := assets.SkillNames()[0]
	if err := os.WriteFile(filepath.Join(cfg, "skills", name, "SKILL.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Run(cfg, allPresent)
	var st SkillStatus
	for _, s := range r.Skills {
		if s.Name == name {
			st = s
		}
	}
	if !st.Installed || st.Matches {
		t.Fatalf("drift not reported: %+v", st)
	}
	if !r.Healthy {
		t.Fatal("drift is a warning, not a failure")
	}
	if !strings.Contains(r.String(), "differs from the embedded version") {
		t.Fatal("text report should describe the drift")
	}
}

func TestDoctorOptionalCapabilityDegradesExplicitly(t *testing.T) {
	cfg := synced(t)
	noCodegraph := func(name string) (string, error) {
		if name == "codegraph" {
			return "", errors.New("not found")
		}
		return allPresent(name)
	}
	r := Run(cfg, noCodegraph)
	if !r.Healthy {
		t.Fatalf("an optional tool must not fail the verdict: %v", r.Problems)
	}
	if !strings.Contains(r.String(), "git ls-files") {
		t.Fatal("report should say how inventory degrades without CodeGraph")
	}
}

// The sandbox is the one flow that needs a container runtime, and a machine without docker has to learn
// that from doctor rather than from a row that ran on this machine. The tool is optional, so its absence is
// a degradation and never a problem: the plan's rows still run on this machine, but a row pinned in a container
// cannot be checked without one.
func TestDoctorSaysHowTheSandboxDegradesWithoutDocker(t *testing.T) {
	cfg := synced(t)
	noDocker := func(name string) (string, error) {
		if name == "docker" {
			return "", errors.New("not found")
		}
		return allPresent(name)
	}
	r := Run(cfg, noDocker)
	if !r.Healthy {
		t.Fatalf("an optional tool must not fail the verdict: %v", r.Problems)
	}
	if !strings.Contains(r.String(), "--sandbox") {
		t.Fatal("report should say that plan admit --sandbox degrades without docker")
	}
}

func TestDoctorJSONShape(t *testing.T) {
	r := Run(synced(t), allPresent)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"config_dir", "skills", "hook_wired", "capabilities", "problems", "healthy"} {
		if _, ok := m[key]; !ok {
			t.Errorf("json report lacks %q", key)
		}
	}
	if m["problems"] == nil {
		t.Fatal("problems must serialize as an array, never null")
	}
}

// A Stop hook that runs a standalone gate binary is a wired gate under another name, not a
// missing gate: reporting "action required" for a working setup trains people to ignore doctor.
func TestHookWiredRecognisesAStandaloneGateBinary(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		wantKind string
	}{
		{"rdd-plus subcommand", `"/home/u/go/bin/rdd-plus" gate`, HookRddPlus},
		{"rdd-plus with flags", `/home/u/go/bin/rdd-plus gate --config-dir /home/u/.claude`, HookRddPlus},
		{"standalone binary", `"/home/u/.claude/hooks/bin/testing-gate"`, HookStandalone},
		{"standalone node script", `node /home/u/.claude/hooks/testing-gate.mjs`, HookStandalone},
		{"an unrelated hook", `gentle-ai review stop-hook --agent claude-code`, HookNone},
		{"a command merely mentioning the word", `echo "run the gate later"`, HookNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			settings := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":` + jsonString(tc.command) + `}]}]}}`
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
				t.Fatal(err)
			}
			kind, cmd := hookWired(filepath.Join(dir, "settings.json"))
			if kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tc.wantKind)
			}
			if kind != HookNone && cmd != tc.command {
				t.Fatalf("command = %q", cmd)
			}
		})
	}
}

// Only a missing gate is a problem; a gate under another name is reported, not flagged.
func TestStandaloneGateIsHealthyAndNamed(t *testing.T) {
	dir := t.TempDir()
	settings := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"/h/.claude/hooks/bin/testing-gate"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Run(dir, func(string) (string, error) { return "/usr/bin/x", nil })
	for _, p := range r.Problems {
		if strings.Contains(p, "gate hook not wired") {
			t.Fatalf("a wired standalone gate must not be a problem: %v", r.Problems)
		}
	}
	if !r.HookWired || r.HookKind != HookStandalone {
		t.Fatalf("wired = %v kind = %q", r.HookWired, r.HookKind)
	}
	if !strings.Contains(r.String(), "standalone gate binary") {
		t.Fatalf("the report must name what is wired:\n%s", r.String())
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// One rule reads every command stored as one string, so the splitter is tested where it lives. A quoted
// path is one word, and both quote characters count as quoting: the reader that used to live beside the
// probe honoured double quotes only, which is how one command read as something other than what a shell
// would run. The signal is tested with the words: a quote left open is reported, not silently repaired,
// because the caller that executes the words has to refuse a command nobody can read — but the words read
// so far are still returned, so a caller that only inspects a command keeps reading exactly what it did
// before the signal existed.
func TestShellWordsSplitsLikeAShell(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"plain command", `/h/bin/rdd-plus gate`, []string{"/h/bin/rdd-plus", "gate"}, false},
		{"double quoted path with a space", `"/h/my bin/rdd-plus" gate`, []string{"/h/my bin/rdd-plus", "gate"}, false},
		{"single quoted path with a space", `'/h/my bin/rdd-plus' gate`, []string{"/h/my bin/rdd-plus", "gate"}, false},
		{"tabs and newlines separate words", "a\tb\r\nc", []string{"a", "b", "c"}, false},
		{"leading and repeated whitespace", "  spaced   out  ", []string{"spaced", "out"}, false},
		{"one quoted word alone", `"/h/gate"`, []string{"/h/gate"}, false},
		{"whitespace only", "   \t\n", nil, false},
		{"empty", "", nil, false},
		// A quote left open is not a command: the probe must not run "unbalanced" as a program.
		{"unterminated double quote", `"/h/my bin/rdd-plus gate`, []string{"/h/my bin/rdd-plus gate"}, true},
		{"unterminated single quote", `'/h/my bin/rdd-plus`, []string{"/h/my bin/rdd-plus"}, true},
		{"unterminated quote after a word", `/h/bin/rdd-plus "gate`, []string{"/h/bin/rdd-plus", "gate"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ShellWords(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ShellWords(%q) error = %v, want an error: %v", tc.in, err, tc.wantErr)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("%q -> %q, want %q", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("%q -> %q, want %q", tc.in, got, tc.want)
				}
			}
		})
	}
}

// writeSettings wires one Stop hook command into a config dir, the way sync would.
func writeSettings(t *testing.T, dir, command string) {
	t.Helper()
	settings := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":` + jsonString(command) + `}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
}

// binary writes a stand-in for the rdd-plus executable.
func binary(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// Today's real layout points the hook and PATH at the same binary through different paths, one of
// them a symlink. Two paths, one file, one verdict: this must stay quiet.
func TestHookBinaryMatchingPathBinaryIsQuiet(t *testing.T) {
	real := binary(t, t.TempDir(), "rdd-plus")
	link := filepath.Join(t.TempDir(), "rdd-plus-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		hook, path string
	}{
		{"same absolute path", real, real},
		{"hook is a symlink to the same file", link, real},
		{"PATH is a symlink to the same file", real, link},
		{"both are symlinks to the same file", link, link},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSettings(t, dir, `"`+tc.hook+`" gate`)
			r := Run(dir, func(name string) (string, error) {
				if name == "rdd-plus" {
					return tc.path, nil
				}
				return "/usr/bin/" + name, nil
			})
			if r.BinariesDiffer {
				t.Fatalf("same file, different path, must not warn: %+v", r)
			}
			if strings.Contains(r.String(), "different verdicts") {
				t.Fatalf("report = %s", r.String())
			}
		})
	}
}

// Two real binaries can give two verdicts: the hook must run the same file the user gets from PATH.
func TestDoctorFlagsAHookBinaryThatDiffersFromPATH(t *testing.T) {
	dir := t.TempDir()
	hookBin := binary(t, t.TempDir(), "rdd-plus")
	pathBin := binary(t, t.TempDir(), "rdd-plus")
	writeSettings(t, dir, `"`+hookBin+`" gate`)
	r := Run(dir, func(name string) (string, error) {
		if name == "rdd-plus" {
			return pathBin, nil
		}
		return "/usr/bin/" + name, nil
	})
	if !r.BinariesDiffer || r.WiredBinary != hookBin || r.PathBinary != pathBin {
		t.Fatalf("report = %+v", r)
	}
	out := r.String()
	for _, want := range []string{hookBin, pathBin, "different verdicts"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{hookBin, pathBin} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("json report must carry both paths, missing %q:\n%s", want, raw)
		}
	}
}

// A quoted command is one command, not its first word: a hook wired from a path with a space in it was
// read as its truncated head, which resolved to nothing, so doctor said nothing about a hook that does
// point at a different binary.
func TestQuotedHookPathWithASpaceIsComparedWhole(t *testing.T) {
	dir := t.TempDir()
	spaceDir := filepath.Join(t.TempDir(), "Program Files")
	if err := os.MkdirAll(spaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookBin := binary(t, spaceDir, "rdd-plus")
	pathBin := binary(t, t.TempDir(), "rdd-plus")
	writeSettings(t, dir, `"`+hookBin+`" gate`)
	r := Run(dir, func(name string) (string, error) {
		if name == "rdd-plus" {
			return pathBin, nil
		}
		return "/usr/bin/" + name, nil
	})
	if !r.BinariesDiffer || r.WiredBinary != hookBin || r.PathBinary != pathBin {
		t.Fatalf("a quoted path with a space must be compared as one path: %+v", r)
	}
}

// Doctor already has verdicts for an unwired hook and for rdd-plus off PATH; the new line must not
// reinvent them.
func TestDoctorStaysQuietWhenItCannotCompareBinaries(t *testing.T) {
	if r := Run(t.TempDir(), allPresent); r.BinariesDiffer || strings.Contains(r.String(), "different verdicts") {
		t.Fatalf("an unwired hook must stay quiet: %+v", r)
	}

	dir := t.TempDir()
	writeSettings(t, dir, `"`+binary(t, t.TempDir(), "rdd-plus")+`" gate`)
	noRddPlus := func(name string) (string, error) {
		if name == "rdd-plus" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
	r := Run(dir, noRddPlus)
	if r.BinariesDiffer || strings.Contains(r.String(), "different verdicts") {
		t.Fatalf("rdd-plus off PATH must stay quiet: %+v", r)
	}
}

// A tree the doctor cannot walk is not a match. The comparison used to swallow that error and report an
// unreadable skill as identical — a health check answering "verified" about files nothing read — and the
// fail-closed half is this function's, because the shared reading already answers false.
func TestDoctorDoesNotCallASkillVerifiedWhenItCannotReadTheTree(t *testing.T) {
	// An empty embedded tree: the skill is not in it, so the walk cannot be completed.
	skills := os.DirFS(t.TempDir())
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if matches(skills, "demo", target) {
		t.Fatal("a skill whose tree could not be walked was reported as identical")
	}
}
