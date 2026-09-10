// Package doctor reports whether rdd-plus is installed and what the environment can do, so a
// skill can degrade explicitly instead of failing on a tool it assumed.
package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alesierraalta/rdd-plus/internal/assets"
)

// SkillStatus is one embedded skill's presence and freshness in the config dir.
type SkillStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Matches   bool   `json:"matches"`
}

// Capability is one external tool and what the flow loses without it.
type Capability struct {
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	Present  bool   `json:"present"`
	Required bool   `json:"required"`
	Degrades string `json:"degrades,omitempty"`
}

// Report is the doctor's verdict; Healthy is false only for missing git, skills, or hook.
type Report struct {
	ConfigDir    string        `json:"config_dir"`
	Skills       []SkillStatus `json:"skills"`
	HookWired    bool          `json:"hook_wired"`
	HookKind     string        `json:"hook_kind"` // HookRddPlus, HookStandalone or HookNone
	HookCommand  string        `json:"hook_command,omitempty"`
	Capabilities []Capability  `json:"capabilities"`
	Problems     []string      `json:"problems"`
	Healthy      bool          `json:"healthy"`
}

var capabilities = []Capability{
	{Name: "git", Required: true, Degrades: "the gate and the fingerprint cannot run at all"},
	{Name: "claude", Degrades: "the eval harness cannot drive sessions"},
	{Name: "node", Degrades: "fixtures that use node:test cannot run"},
	{Name: "python3", Degrades: "the eval harness and seed-mutants (still Python) cannot run"},
	{Name: "codegraph", Degrades: "inventory falls back to git ls-files plus grep"},
	{Name: "rtk", Degrades: "shell output is not compacted; nothing else changes"},
	{Name: "gentle-ai", Degrades: "no native review lifecycle; receipts are never emitted"},
	{Name: "engram", Degrades: "the plan file is the only memory across sessions"},
}

// Run inspects cfgDir and PATH (through lookPath, injectable for tests).
func Run(cfgDir string, lookPath func(string) (string, error)) Report {
	r := Report{ConfigDir: cfgDir, Problems: []string{}}
	skills := assets.Skills()
	for _, name := range assets.SkillNames() {
		st := SkillStatus{Name: name}
		target := filepath.Join(cfgDir, "skills", name)
		if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err == nil {
			st.Installed = true
			st.Matches = matches(skills, name, target)
		}
		r.Skills = append(r.Skills, st)
		if !st.Installed {
			r.Problems = append(r.Problems, "skill not installed: "+name+" (run: rdd-plus sync)")
		}
	}
	r.HookKind, r.HookCommand = hookWired(filepath.Join(cfgDir, "settings.json"))
	r.HookWired = r.HookKind != HookNone
	if !r.HookWired {
		r.Problems = append(r.Problems, "gate hook not wired in settings.json (run: rdd-plus sync)")
	}
	for _, c := range capabilities {
		if p, err := lookPath(c.Name); err == nil {
			c.Present, c.Path = true, p
		}
		if c.Required && !c.Present {
			r.Problems = append(r.Problems, "required tool missing: "+c.Name)
		}
		r.Capabilities = append(r.Capabilities, c)
	}
	r.Healthy = len(r.Problems) == 0
	return r
}

// String renders the report for a terminal.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "rdd-plus doctor · config dir %s\n\nskills\n", r.ConfigDir)
	for _, s := range r.Skills {
		state := "missing"
		if s.Installed && s.Matches {
			state = "ok"
		} else if s.Installed {
			state = "installed, differs from the embedded version"
		}
		fmt.Fprintf(&b, "  %-32s %s\n", s.Name, state)
	}
	fmt.Fprintf(&b, "\nhook\n")
	switch r.HookKind {
	case HookRddPlus:
		fmt.Fprintf(&b, "  Stop gate wired: %s\n", r.HookCommand)
	case HookStandalone:
		fmt.Fprintf(&b, "  Stop gate wired to a standalone gate binary: %s\n", r.HookCommand)
	default:
		fmt.Fprintf(&b, "  Stop gate NOT wired\n")
	}
	fmt.Fprintf(&b, "\ncapabilities\n")
	for _, c := range r.Capabilities {
		mark := "present"
		if !c.Present {
			mark = "absent"
			if c.Required {
				mark = "ABSENT (required)"
			}
		}
		fmt.Fprintf(&b, "  %-10s %-18s", c.Name, mark)
		if !c.Present && c.Degrades != "" {
			fmt.Fprintf(&b, " without it: %s", c.Degrades)
		}
		b.WriteString("\n")
	}
	if r.Healthy {
		fmt.Fprintf(&b, "\nverdict: healthy\n")
	} else {
		fmt.Fprintf(&b, "\nverdict: action required\n")
		for _, p := range r.Problems {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	}
	return b.String()
}

func matches(skills fs.FS, name, target string) bool {
	same := true
	_ = fs.WalkDir(skills, name, func(p string, d fs.DirEntry, err error) error {
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
	return same
}

// hookWired reports whether any Stop hook command ends with " gate" and returns it.
// What a Stop hook runs: this binary's own subcommand, a separately built gate, or nothing.
const (
	HookRddPlus    = "rdd-plus"
	HookStandalone = "standalone"
	HookNone       = "none"
)

// gateBinaryRe recognises a standalone build of the gate by the name it is installed under.
var gateBinaryRe = regexp.MustCompile(`(^|[/\\"' ])testing-gate(\.mjs|\.js)?("|'|$|\s)`)

// hookWired reports what the Stop hook runs. A gate installed under its own name counts: the
// question is whether a gate runs at the Stop, not whether this binary is the one running it.
func hookWired(settingsPath string) (string, string) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return HookNone, ""
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		return HookNone, ""
	}
	hooks, _ := s["hooks"].(map[string]any)
	stop, _ := hooks["Stop"].([]any)
	for _, e := range stop {
		entry, _ := e.(map[string]any)
		list, _ := entry["hooks"].([]any)
		for _, h := range list {
			hook, _ := h.(map[string]any)
			cmd, _ := hook["command"].(string)
			trimmed := strings.TrimSpace(cmd)
			if trimmed == "" {
				continue
			}
			// "gate" counts only in the subcommand position, right after the executable.
			if fields := strings.Fields(trimmed); len(fields) > 1 && fields[1] == "gate" {
				return HookRddPlus, cmd
			}
			if gateBinaryRe.MatchString(trimmed) {
				return HookStandalone, cmd
			}
		}
	}
	return HookNone, ""
}
