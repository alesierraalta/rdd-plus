// Package sync installs the embedded skills into every discovered host and wires the gate
// as a Claude Stop hook, merging into settings.json without touching anything it does not own.
package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

// Options controls a sync run.
type Options struct {
	DryRun bool
}

// HostReport says what sync did, or would do, for one host.
type HostReport struct {
	Host            Host
	Written         []string
	Unchanged       []string
	BackedUp        map[string]string
	RemovedHooks    []string
	SettingsPath    string
	SettingsRead    bool
	SettingsChanged bool
}

// Report says what a sync did, or would do under DryRun. The fields copied from the Claude
// host remain for callers of Sync; Hosts is the multi-host report used by the CLI.
type Report struct {
	ConfigDir    string
	DryRun       bool
	Written      []string
	Unchanged    []string
	BackedUp     map[string]string
	RemovedHooks []string
	SettingsPath string
	// SettingsRead is false when settings.json could not be read, and then the report says nothing about
	// wiring it: a report is printed before the error that explains a refusal, so a line claiming the hook
	// is `already wired` would be a sentence about a file nobody looked at.
	SettingsRead    bool
	SettingsChanged bool

	Hosts           []HostReport
	LookedFor       []string
	DiscoveryErrors []string
	ClaudeConfigDir string
}

var documentedOnlyHosts = []string{"opencode", "gemini", "codex"}

// String renders the report for a terminal.
func (r Report) String() string {
	var b strings.Builder
	prefix := ""
	if r.DryRun {
		prefix = "[dry-run] "
	}
	if len(r.Hosts) == 0 {
		fmt.Fprintf(&b, "%sno installed hosts found\n", prefix)
		if len(r.LookedFor) > 0 {
			fmt.Fprintf(&b, "%slooked for host config dirs:\n", prefix)
			for _, path := range r.LookedFor {
				fmt.Fprintf(&b, "%s  %s\n", prefix, path)
			}
		}
	} else {
		for _, host := range r.Hosts {
			fmt.Fprintf(&b, "%shost: %s\n", prefix, host.Host.Name)
			fmt.Fprintf(&b, "%sconfig dir: %s\n", prefix, host.Host.ConfigDir)
			for _, s := range host.Written {
				if dir, ok := host.BackedUp[s]; ok {
					fmt.Fprintf(&b, "%sskill %-32s replaced (previous copy at %s)\n", prefix, s, dir)
				} else {
					fmt.Fprintf(&b, "%sskill %-32s written\n", prefix, s)
				}
			}
			for _, s := range host.Unchanged {
				fmt.Fprintf(&b, "%sskill %-32s unchanged\n", prefix, s)
			}
			for _, h := range host.RemovedHooks {
				fmt.Fprintf(&b, "%sremoved previous gate hook: %s\n", prefix, h)
			}
			if host.SettingsRead {
				if host.SettingsChanged {
					fmt.Fprintf(&b, "%ssettings: gate hook wired in %s\n", prefix, host.SettingsPath)
				} else {
					fmt.Fprintf(&b, "%ssettings: gate hook already wired in %s\n", prefix, host.SettingsPath)
				}
			}
		}
	}
	for _, problem := range r.DiscoveryErrors {
		fmt.Fprintf(&b, "%sdiscovery: %s\n", prefix, problem)
	}

	claudeRead := false
	claudePresent := false
	for _, host := range r.Hosts {
		if host.Host.Name == "claude" {
			claudePresent = true
			claudeRead = host.SettingsRead
			break
		}
	}
	if claudeRead || !claudePresent {
		claudeDir := r.ClaudeConfigDir
		if claudeDir == "" {
			claudeDir = r.ConfigDir
		}
		if claudeRead {
			fmt.Fprintf(&b, "%shook: Stop hook wired only in Claude config dir %s\n", prefix, claudeDir)
		} else {
			fmt.Fprintf(&b, "%shook: Stop hook not wired; Claude config dir %s was not discovered\n", prefix, claudeDir)
		}
		fmt.Fprintf(&b, "%shook transport: documented rather than wired for %s\n", prefix, strings.Join(documentedOnlyHosts, ", "))
	}
	return b.String()
}

// HookCommand is the exact command sync wires for a given binary path.
func HookCommand(binPath string) string {
	return fmt.Sprintf("%q gate", binPath)
}

// Sync installs every embedded skill under the Claude config directory and wires its Stop hook.
// It preserves the original single-directory API; the CLI uses SyncHosts after discovery.
func Sync(cfgDir, binPath string, opts Options) (Report, error) {
	return SyncHosts([]Host{{Name: "claude", ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}}, binPath, opts)
}

// SyncHosts installs every embedded skill into each supplied host. Only the host named
// claude receives settings.json hook wiring; all other hosts receive skills only.
func SyncHosts(hosts []Host, binPath string, opts Options) (Report, error) {
	report := Report{DryRun: opts.DryRun, BackedUp: map[string]string{}}
	for _, host := range hosts {
		if host.Name == "claude" {
			report.ConfigDir = host.ConfigDir
			report.ClaudeConfigDir = host.ConfigDir
		}
		hostReport, err := syncHost(host, binPath, opts)
		report.Hosts = append(report.Hosts, hostReport)
		if host.Name == "claude" {
			report.ConfigDir = host.ConfigDir
			report.Written = hostReport.Written
			report.Unchanged = hostReport.Unchanged
			report.BackedUp = hostReport.BackedUp
			report.RemovedHooks = hostReport.RemovedHooks
			report.SettingsPath = hostReport.SettingsPath
			report.SettingsRead = hostReport.SettingsRead
			report.SettingsChanged = hostReport.SettingsChanged
		}
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func syncHost(host Host, binPath string, opts Options) (HostReport, error) {
	report := HostReport{Host: host, BackedUp: map[string]string{}}
	var settings map[string]any
	var raw []byte
	var err error
	if host.Name == "claude" {
		report.SettingsPath = filepath.Join(host.ConfigDir, "settings.json")
		settings, raw, err = loadSettings(report.SettingsPath)
		if err != nil {
			return report, err
		}
		report.SettingsRead = true
	}

	skills := assets.Skills()
	for _, name := range assets.SkillNames() {
		target := filepath.Join(host.SkillsDir, name)
		same, err := identical(skills, name, target)
		if err != nil {
			return report, err
		}
		if same {
			report.Unchanged = append(report.Unchanged, name)
			continue
		}
		if _, err := os.Stat(target); err == nil {
			backup := nextBackupPath(host.SkillsDir, name)
			report.BackedUp[name] = backup
			if !opts.DryRun {
				if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
					return report, err
				}
				if err := os.Rename(target, backup); err != nil {
					return report, err
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return report, err
		}
		if !opts.DryRun {
			if err := writeSkill(skills, name, target); err != nil {
				return report, err
			}
		}
		report.Written = append(report.Written, name)
	}

	if host.Name != "claude" {
		return report, nil
	}
	changed, removed := wireHook(settings, HookCommand(binPath))
	report.RemovedHooks = removed
	report.SettingsChanged = changed || raw == nil
	if report.SettingsChanged && !opts.DryRun {
		if err := os.MkdirAll(host.ConfigDir, 0o755); err != nil {
			return report, err
		}
		out, err := marshalSettings(settings)
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(report.SettingsPath, out, 0o644); err != nil {
			return report, err
		}
	}
	return report, nil
}

func nextBackupPath(skillsDir, name string) string {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := filepath.Join(skillsDir, ".rdd-plus-backup", name+"-"+stamp)
	candidate := base
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) || err != nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// loadSettings returns the parsed settings, the raw bytes (nil when the file is absent),
// and an error when the file exists but is not a JSON object.
func loadSettings(path string) (map[string]any, []byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil || settings == nil {
		return nil, nil, fmt.Errorf("%s is not a JSON object; nothing was written", path)
	}
	return settings, raw, nil
}

func marshalSettings(settings map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// identical reports whether every embedded file of the skill exists at target with the same
// bytes; files the user added next to them (run artifacts, notes) do not count as a difference.
func identical(skills fs.FS, name, target string) (bool, error) {
	if _, err := os.Stat(target); err != nil {
		return false, nil
	}
	same := true
	err := fs.WalkDir(skills, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !same {
			return err
		}
		want, err := fs.ReadFile(skills, p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(name, filepath.FromSlash(p))
		got, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil || !bytes.Equal(want, got) {
			same = false
		}
		return nil
	})
	return same, err
}

func writeSkill(skills fs.FS, name, target string) error {
	return fs.WalkDir(skills, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(name, filepath.FromSlash(p))
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(skills, p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, ".sh") || strings.HasSuffix(p, ".py") {
			mode = 0o755
		}
		return os.WriteFile(dst, data, mode)
	})
}

// wireHook makes settings carry exactly one gate hook: previous gate entries (the Node hook,
// the pre-repo binary, or an older rdd-plus path) are dropped, and the desired command is added
// only when absent. It returns whether settings changed and which commands were removed.
func wireHook(settings map[string]any, command string) (bool, []string) {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	stop, _ := hooks["Stop"].([]any)
	changed := false
	var removed []string
	present := false
	kept := make([]any, 0, len(stop))
	for _, e := range stop {
		entry, _ := e.(map[string]any)
		if entry == nil {
			kept = append(kept, e)
			continue
		}
		list, _ := entry["hooks"].([]any)
		keptHooks := make([]any, 0, len(list))
		for _, h := range list {
			hook, _ := h.(map[string]any)
			cmd, _ := hook["command"].(string)
			switch {
			case cmd == command:
				present = true
				keptHooks = append(keptHooks, h)
			case isPreviousGate(cmd):
				removed = append(removed, cmd)
				changed = true
			default:
				keptHooks = append(keptHooks, h)
			}
		}
		if len(keptHooks) == 0 && len(list) > 0 {
			continue // an entry that only carried a previous gate goes away with it
		}
		entry["hooks"] = keptHooks
		kept = append(kept, entry)
	}
	if !present {
		kept = append(kept, map[string]any{
			"matcher": "",
			"hooks": []any{map[string]any{
				"type":          "command",
				"command":       command,
				"timeout":       30,
				"statusMessage": "Checking testing discipline...",
			}},
		})
		changed = true
	}
	hooks["Stop"] = kept
	return changed, removed
}

// isPreviousGate recognizes every earlier way the gate was wired.
func isPreviousGate(cmd string) bool {
	c := strings.TrimSpace(cmd)
	c = strings.TrimSuffix(c, " gate")
	c = strings.Trim(c, "\"")
	return strings.HasSuffix(c, "/testing-gate.mjs") ||
		strings.HasSuffix(c, "/hooks/bin/testing-gate") ||
		(strings.HasSuffix(cmd, " gate") && strings.Contains(c, "rdd-plus"))
}
