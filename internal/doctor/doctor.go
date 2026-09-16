// Package doctor reports whether rdd-plus is installed and what the environment can do, so a
// skill can degrade explicitly instead of failing on a tool it assumed.
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alesierraalta/rdd-plus/internal/assets"
	"github.com/alesierraalta/rdd-plus/internal/skilltree"
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
	ConfigDir      string        `json:"config_dir"`
	Skills         []SkillStatus `json:"skills"`
	HookWired      bool          `json:"hook_wired"`
	HookKind       string        `json:"hook_kind"`   // HookRddPlus, HookStandalone or HookNone
	HookProbed     bool          `json:"hook_probed"` // the wired command answered a hook payload
	HookCommand    string        `json:"hook_command,omitempty"`
	WiredBinary    string        `json:"wired_binary,omitempty"` // the binary the wired Stop hook invokes
	PathBinary     string        `json:"path_binary,omitempty"`  // where rdd-plus resolves on PATH
	BinariesDiffer bool          `json:"binaries_differ"`        // both exist and are different files
	Capabilities   []Capability  `json:"capabilities"`
	Problems       []string      `json:"problems"`
	Healthy        bool          `json:"healthy"`
}

var capabilities = []Capability{
	{Name: "git", Required: true, Degrades: "the gate and the fingerprint cannot run at all"},
	{Name: "claude", Degrades: "the eval harness cannot drive sessions"},
	{Name: "node", Degrades: "fixtures that use node:test cannot run"},
	{Name: "python3", Degrades: "the eval harness and seed-mutants (still Python) cannot run"},
	{Name: "docker", Degrades: "plan admit --sandbox has no container to observe in, so a sandbox pin cannot be checked and a mutation cannot be replayed"},
	{Name: "codegraph", Degrades: "inventory falls back to git ls-files plus grep"},
	{Name: "rtk", Degrades: "shell output is not compacted; nothing else changes"},
	{Name: "gentle-ai", Degrades: "no native review lifecycle; receipts are never emitted"},
	{Name: "engram", Degrades: "the plan file is the only memory across sessions"},
}

// Run inspects cfgDir and PATH (through lookPath, injectable for tests).
// Run reports on a configuration without executing anything from it.
func Run(cfgDir string, lookPath func(string) (string, error)) Report {
	return RunWith(cfgDir, lookPath, nil)
}

// RunWith adds a probe: matching the shape of a hook command says nothing about whether running
// it works, and a doctor that reports a broken wiring as healthy is worse than no doctor.
func RunWith(cfgDir string, lookPath func(string) (string, error), probe func(command string) error) Report {
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
	} else {
		r.WiredBinary, r.PathBinary, r.BinariesDiffer = compareGateBinaries(r.HookCommand, lookPath)
		if probe != nil {
			if err := probe(r.HookCommand); err != nil {
				r.Problems = append(r.Problems, "the wired command does not answer a hook payload: "+err.Error())
			} else {
				r.HookProbed = true
			}
		}
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
	if r.HookProbed {
		fmt.Fprintf(&b, "  the wired command answers a payload\n")
	}
	if r.BinariesDiffer {
		fmt.Fprintf(&b, "  warning: the wired gate binary %s and the rdd-plus on PATH %s are different files: the two would give different verdicts\n", r.WiredBinary, r.PathBinary)
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

// matches reports whether the installed skill still holds the bytes this build shipped.
//
// A tree it cannot walk is not a match. The copy this replaced swallowed that error and answered "identical"
// for a skill nothing had read — the one answer a check must never give about a file it did not read. The error
// itself needs no plumbing here: a health report answers yes or no, and the shared reading already returns false
// beside it, so the doctor's half is only the decision.
func matches(skills fs.FS, name, target string) bool {
	same, _ := skilltree.Identical(skills, name, target)
	return same
}

// ShellWords splits a command the way a shell would: on whitespace, except inside quotes, where a space
// is part of the word. Every reader of a command stored as one string goes through here — the wiring
// check, the binary comparison, the probe that runs the wired hook, and a bench suite command — so one
// rule reads one field. It also reports whether the command was well formed: a quote left open is not a
// command anyone can read, so the caller that executes the words must refuse it, while a caller that only
// inspects a command reads the words either way. Splitting on whitespace alone, a hook wired as
// `"/opt/Program Files/rdd-plus" gate` reads as three words whose first is a truncated path: the wiring
// check misses the subcommand, the comparison resolves nothing, and doctor stays quiet about a hook it is
// there to judge.
func ShellWords(command string) ([]string, error) {
	var (
		words []string
		cur   strings.Builder
		quote byte
	)
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(command); i++ {
		switch c := command[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	if quote != 0 {
		return words, errors.New("unterminated quote")
	}
	return words, nil
}

// compareGateBinaries answers whether the wired Stop hook runs a different file than the rdd-plus on
// PATH. Two paths that resolve to the same file (a symlink, today's real layout) are one binary and
// one verdict; two different files are a time bomb. A missing hook binary or a missing PATH binary
// is left to the verdicts doctor already reports.
func compareGateBinaries(hookCommand string, lookPath func(string) (string, error)) (wired, path string, differ bool) {
	words, _ := ShellWords(hookCommand)
	if len(words) == 0 {
		return "", "", false
	}
	wired = words[0]
	// Only a command written as a path names a binary doctor can resolve; `node script.mjs` and a
	// bare `rdd-plus` are left alone rather than guessed at.
	if !strings.ContainsRune(wired, filepath.Separator) {
		return "", "", false
	}
	wiredInfo, err := os.Stat(wired)
	if err != nil || wiredInfo.IsDir() {
		return "", "", false
	}
	p, err := lookPath("rdd-plus")
	if err != nil || p == "" {
		return "", "", false
	}
	pathInfo, err := os.Stat(p)
	if err != nil || pathInfo.IsDir() {
		return "", "", false
	}
	if os.SameFile(wiredInfo, pathInfo) {
		return wired, p, false
	}
	return wired, p, true
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
			if fields, _ := ShellWords(trimmed); len(fields) > 1 && fields[1] == "gate" {
				return HookRddPlus, cmd
			}
			if gateBinaryRe.MatchString(trimmed) {
				return HookStandalone, cmd
			}
		}
	}
	return HookNone, ""
}
