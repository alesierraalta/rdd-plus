package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

const bin = "/opt/tools/rdd-plus"

func readSettings(t *testing.T, cfg string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(cfg, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("settings.json is not JSON: %v", err)
	}
	return s
}

func stopCommands(t *testing.T, s map[string]any) []string {
	t.Helper()
	var out []string
	hooks, _ := s["hooks"].(map[string]any)
	stop, _ := hooks["Stop"].([]any)
	for _, e := range stop {
		entry := e.(map[string]any)
		for _, h := range entry["hooks"].([]any) {
			out = append(out, h.(map[string]any)["command"].(string))
		}
	}
	return out
}

func TestSyncFreshConfigDir(t *testing.T) {
	cfg := t.TempDir()
	report, err := Sync(cfg, bin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Written) != len(assets.SkillNames()) {
		t.Fatalf("written %d skills, want %d", len(report.Written), len(assets.SkillNames()))
	}
	for _, name := range assets.SkillNames() {
		if _, err := os.Stat(filepath.Join(cfg, "skills", name, "SKILL.md")); err != nil {
			t.Errorf("skill %s not installed: %v", name, err)
		}
	}
	if !report.SettingsChanged {
		t.Fatal("settings should be created on a fresh dir")
	}
	cmds := stopCommands(t, readSettings(t, cfg))
	if len(cmds) != 1 || cmds[0] != HookCommand(bin) {
		t.Fatalf("stop hooks = %q, want exactly %q", cmds, HookCommand(bin))
	}
}

func TestSyncPreservesExistingHooksAndSettings(t *testing.T) {
	cfg := t.TempDir()
	existing := `{
  "model": "opus",
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "rtk hook claude"}]}],
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "gentle-ai review stop-hook --agent claude-code", "timeout": 60}]}]
  }
}`
	if err := os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(cfg, bin, Options{}); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, cfg)
	if s["model"] != "opus" {
		t.Errorf("unrelated setting lost: model=%v", s["model"])
	}
	pre, _ := s["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Errorf("PreToolUse hooks lost: %v", pre)
	}
	cmds := stopCommands(t, s)
	if len(cmds) != 2 || cmds[0] != "gentle-ai review stop-hook --agent claude-code" || cmds[1] != HookCommand(bin) {
		t.Fatalf("stop hooks = %q", cmds)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	cfg := t.TempDir()
	if _, err := Sync(cfg, bin, Options{}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(cfg, "settings.json"))
	report, err := Sync(cfg, bin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Written) != 0 || len(report.Unchanged) != len(assets.SkillNames()) {
		t.Fatalf("second run wrote %v, unchanged %d", report.Written, len(report.Unchanged))
	}
	if report.SettingsChanged {
		t.Fatal("second run must not change settings")
	}
	after, _ := os.ReadFile(filepath.Join(cfg, "settings.json"))
	if string(before) != string(after) {
		t.Fatal("settings.json bytes changed on an idempotent run")
	}
	if len(stopCommands(t, readSettings(t, cfg))) != 1 {
		t.Fatal("gate hook duplicated")
	}
}

func TestSyncBacksUpADifferingSkill(t *testing.T) {
	cfg := t.TempDir()
	if _, err := Sync(cfg, bin, Options{}); err != nil {
		t.Fatal(err)
	}
	name := assets.SkillNames()[0]
	skillMD := filepath.Join(cfg, "skills", name, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Sync(cfg, bin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	backup, ok := report.BackedUp[name]
	if !ok {
		t.Fatalf("expected %s to be backed up; report %+v", name, report)
	}
	saved, err := os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if err != nil || string(saved) != "local edit\n" {
		t.Fatalf("backup does not hold the local edit: %v %q", err, saved)
	}
	restored, _ := os.ReadFile(skillMD)
	if string(restored) == "local edit\n" {
		t.Fatal("skill was not replaced with the embedded version")
	}
	if !strings.Contains(backup, ".rdd-plus-backup") {
		t.Fatalf("backup dir %s is not under .rdd-plus-backup", backup)
	}
}

func TestSyncRefusesInvalidSettingsAndWritesNothing(t *testing.T) {
	cfg := t.TempDir()
	broken := []byte("{not json")
	if err := os.WriteFile(filepath.Join(cfg, "settings.json"), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Sync(cfg, bin, Options{})
	if err == nil {
		t.Fatal("expected an error on invalid settings.json")
	}
	if _, err := os.Stat(filepath.Join(cfg, "skills")); err == nil {
		t.Fatal("skills were written despite invalid settings")
	}
	after, _ := os.ReadFile(filepath.Join(cfg, "settings.json"))
	if string(after) != string(broken) {
		t.Fatal("settings.json was modified")
	}
}

// A report is printed before the error that explains a refusal, so it must not claim a state it never read:
// telling a reader the gate hook is `already wired` in a file the tool could not parse is a sentence about a
// file nobody looked at, and it sits one line above the refusal that says otherwise.
func TestSyncSaysNothingAboutTheHookInSettingsItCouldNotRead(t *testing.T) {
	cfg := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Sync(cfg, bin, Options{})
	if err == nil {
		t.Fatal("expected an error on invalid settings.json")
	}
	if got := report.String(); strings.Contains(got, "gate hook") {
		t.Fatalf("the report claims a hook state it never read:\n%s", got)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	cfg := t.TempDir()
	report, err := Sync(cfg, bin, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || len(report.Written) == 0 {
		t.Fatalf("dry-run report should plan writes: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(cfg, "skills")); err == nil {
		t.Fatal("dry-run wrote skills")
	}
	if _, err := os.Stat(filepath.Join(cfg, "settings.json")); err == nil {
		t.Fatal("dry-run wrote settings.json")
	}
	if !strings.Contains(report.String(), "[dry-run]") {
		t.Fatal("report should be marked as dry-run")
	}
}

func TestSyncRemovesPreviousGateHooks(t *testing.T) {
	cfg := t.TempDir()
	existing := `{"hooks":{"Stop":[
  {"matcher":"","hooks":[{"type":"command","command":"gentle-ai review stop-hook --agent claude-code","timeout":60}]},
  {"matcher":"","hooks":[{"type":"command","command":"\"/home/u/.claude/hooks/testing-gate.mjs\"","timeout":30}]},
  {"matcher":"","hooks":[{"type":"command","command":"\"/home/u/.claude/hooks/bin/testing-gate\"","timeout":30}]},
  {"matcher":"","hooks":[{"type":"command","command":"\"/old/place/rdd-plus\" gate","timeout":30}]}
]}}`
	if err := os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Sync(cfg, bin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.RemovedHooks) != 3 {
		t.Fatalf("removed %d previous gate hooks, want 3: %q", len(report.RemovedHooks), report.RemovedHooks)
	}
	cmds := stopCommands(t, readSettings(t, cfg))
	if len(cmds) != 2 || cmds[0] != "gentle-ai review stop-hook --agent claude-code" || cmds[1] != HookCommand(bin) {
		t.Fatalf("stop hooks = %q", cmds)
	}
}

func TestIsPreviousGate(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{`"/home/u/.claude/hooks/testing-gate.mjs"`, true},
		{`"/home/u/.claude/hooks/bin/testing-gate"`, true},
		{`"/old/rdd-plus" gate`, true},
		{`gentle-ai review stop-hook --agent claude-code`, false},
		{`"/home/u/.claude/hooks/ctx-read-guard.mjs"`, false},
		{`some-other-tool gate`, false},
	}
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			if got := isPreviousGate(c.cmd); got != c.want {
				t.Fatalf("isPreviousGate(%q) = %v, want %v", c.cmd, got, c.want)
			}
		})
	}
}
