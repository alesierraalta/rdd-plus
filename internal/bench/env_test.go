package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A benchmark case must measure the testing skills, not whatever else the machine's global
// configuration makes a session do. The agent runs against a config directory the run owns.
func TestMinimalConfigHoldsTheSkillsAndNothingElse(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, CredentialsFile), []byte(`{"token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "cfg")
	if err := WriteBenchConfig(cfg, real); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "skills", "test-strategy", "SKILL.md")); err != nil {
		t.Fatalf("the skills under test must be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("a global CLAUDE.md would add instructions the benchmark is not measuring")
	}
	data, err := os.ReadFile(filepath.Join(cfg, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"hooks", "mcpServers", "enabledPlugins"} {
		if strings.Contains(string(data), unwanted) {
			t.Fatalf("settings must carry no %s: %s", unwanted, data)
		}
	}
	// Without the operator's credentials the agent cannot log in and every case fails at once.
	link := filepath.Join(cfg, CredentialsFile)
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("credentials must be linked, not copied: %v", err)
	}
	if target != filepath.Join(real, CredentialsFile) {
		t.Fatalf("credentials link points at %q", target)
	}
	got, err := os.ReadFile(link)
	if err != nil || string(got) != `{"token":"x"}` {
		t.Fatalf("credentials unreadable through the link: %v %s", err, got)
	}
}

// A config directory built with no credentials to link is still usable; it simply cannot log in,
// which the run reports as a failed case rather than a recall of zero.
func TestBenchConfigWithoutCredentials(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "cfg")
	if err := WriteBenchConfig(cfg, filepath.Join(t.TempDir(), "absent")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(cfg, CredentialsFile)); !os.IsNotExist(err) {
		t.Fatal("nothing to link, so nothing must be created")
	}
}

// The skill now tells the flow to run `rdd-plus plan init`, so the agent has to be able to find
// the binary that is under test, not whichever one happens to be installed.
func TestAgentEnvPutsTheRunningBinaryFirstOnPath(t *testing.T) {
	env := agentEnv([]string{"PATH=/bin:/usr/bin"}, "/cfg", "/repo/bin")
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			if e != "PATH=/repo/bin:/bin:/usr/bin" {
				t.Fatalf("path = %q", e)
			}
			return
		}
	}
	t.Fatal("no PATH in the agent environment")
}

func TestAgentEnvPointsAtTheGivenConfig(t *testing.T) {
	env := agentEnv([]string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/old", "HOME=/h"}, "/new", "")
	var got []string
	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			got = append(got, e)
		}
	}
	if len(got) != 1 || got[0] != "CLAUDE_CONFIG_DIR=/new" {
		t.Fatalf("config dir env = %v", got)
	}
	if agentEnv([]string{"PATH=/bin"}, "", "") != nil {
		t.Fatal("without a config dir the agent must inherit the environment unchanged")
	}
}
