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
	cfg := filepath.Join(dir, "cfg")
	if err := WriteBenchConfig(cfg); err != nil {
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
