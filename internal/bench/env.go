package bench

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

// WriteBenchConfig builds a Claude configuration directory holding the embedded skills and
// nothing else. A benchmark case must measure the testing skills, not the operator's global
// instructions, memory protocol, or MCP servers, which cost turns and vary between machines.
func WriteBenchConfig(dir string) error {
	skills := filepath.Join(dir, "skills")
	if err := os.RemoveAll(skills); err != nil {
		return err
	}
	err := fs.WalkDir(assets.Skills(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		target := filepath.Join(skills, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(assets.Skills(), p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return err
	}
	settings, err := json.MarshalIndent(map[string]any{"includeCoAuthoredBy": false}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "settings.json"), append(settings, '\n'), 0o644)
}

// agentEnv returns the environment for the agent process: CLAUDE_CONFIG_DIR set to cfgDir,
// replacing any inherited value, and binDir first on PATH so the flow finds the binary under
// test. Empty cfgDir and binDir mean "inherit unchanged".
func agentEnv(base []string, cfgDir, binDir string) []string {
	if cfgDir == "" && binDir == "" {
		return nil
	}
	out := make([]string, 0, len(base)+1)
	for _, e := range base {
		switch {
		case strings.HasPrefix(e, "CLAUDE_CONFIG_DIR="):
			continue
		case binDir != "" && strings.HasPrefix(e, "PATH="):
			out = append(out, "PATH="+binDir+string(os.PathListSeparator)+strings.TrimPrefix(e, "PATH="))
		default:
			out = append(out, e)
		}
	}
	if cfgDir != "" {
		out = append(out, "CLAUDE_CONFIG_DIR="+cfgDir)
	}
	return out
}
