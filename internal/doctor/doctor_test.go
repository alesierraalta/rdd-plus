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
