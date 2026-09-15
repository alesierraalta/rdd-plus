package gate

import (
	"bufio"
	"encoding/json"
	"github.com/alesierraalta/rdd-plus/internal/plan"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A home directory shows up as a git repository with hundreds of thousands of untracked
// entries; a project never does. Above this many entries the checkout is not a project.
const MaxStatusEntries = 20000

// Window used when the transcript carries no timestamp: wide enough to cover any session.
const wideWindow = 8 * time.Hour

// Early transcript lines lack a timestamp; scanning further than this is not a session start.
const maxTimestampScan = 500

var Skills = []string{"test-strategy", "exploit-testing", "no-excess-tests", "real-run-validation"}

var Adversarial = []string{"test-strategy", "exploit-testing"}

var sourceExt = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".go": true, ".rs": true, ".java": true, ".rb": true, ".php": true,
	".cs": true, ".swift": true, ".kt": true, ".scala": true, ".ex": true, ".exs": true,
}

var excludedDirs = map[string]bool{
	"test": true, "tests": true, "spec": true, "__tests__": true, "e2e": true,
	"fixture": true, "fixtures": true, "eval": true, "evals": true,
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true,
	".venv": true, "venv": true, ".cache": true, ".claude": true,
}

var (
	skillCall = regexp.MustCompile(`"skill"\s*:\s*"(` + strings.Join(Skills, "|") + `)"`)
	skillRead = regexp.MustCompile(`skills/(` + strings.Join(Skills, "|") + `)/SKILL\.md`)
)

type Input struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// Deps are the process boundaries: git, the filesystem clock, the transcript, and time.
type Deps struct {
	Git            func(dir string, args ...string) (string, error)
	Stat           func(path string) (os.FileInfo, error)
	OpenTranscript func(path string) (io.ReadCloser, error)
	ReadPlan       func(path string) (string, error)
	Now            time.Time
	WorkDir        string
}

// Entry is one logged decision; skills_loaded is never null so the log stays queryable.
type Entry struct {
	TS            string   `json:"ts"`
	Session       string   `json:"session"`
	Repo          string   `json:"repo"`
	ChangedSource int      `json:"changed_source"`
	SkillsLoaded  []string `json:"skills_loaded"`
	Audited       bool     `json:"audited,omitempty"`
	Fired         bool     `json:"fired"`
	Skipped       string   `json:"skipped,omitempty"`
	OptedOut      bool     `json:"opted_out,omitempty"`
	Plan          string   `json:"plan,omitempty"`
}

type Result struct {
	Fire   bool
	Audit  bool // the discipline ran and left breadth owed; a different question, same Stop
	Reason string
	Files  []string
	Entry  *Entry
	// Owed and Pending are the decision itself, structured: layers assigned and never invoked, and
	// ranked targets still pending. The operator line reads them instead of counting phrases inside
	// Reason — prose this package writes and is free to reword, which is how the line-anchored report
	// silently turned three owed layers into "the plan owes nothing".
	Owed    int
	Pending int
	// Unreadable and Unplanned complete the decision: a cut breadth table and a plan with no layer matrix
	// both make Gaps.Any() true without raising Owed or Pending.
	Unreadable int
	Unplanned  bool
	// Problem is a repository state the gate could not act on: a plan declaration it cannot read. It is
	// not an audit — nothing was read — and it is never silence: the model and the operator both hear it.
	Problem string
}

// IsProductionSource decides what the gate protects: source by extension, outside test,
// fixture, vendored, build, and Claude configuration trees.
func IsProductionSource(path string) bool {
	lower := strings.ToLower(path)
	if !sourceExt[filepath.Ext(lower)] {
		return false
	}
	if strings.HasSuffix(lower, ".d.ts") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return false
	}
	segments := strings.Split(lower, "/")
	for _, dir := range segments[:len(segments)-1] {
		if excludedDirs[dir] {
			return false
		}
	}
	return true
}

// ParsePorcelain reads `git status --porcelain -z -uall`: a rename or copy carries the old
// path as the following field, which is skipped so the file counts once under its new name;
// deletions have nothing on disk to bound to the session.
func ParsePorcelain(raw string) []string {
	fields := strings.Split(raw, "\x00")
	out := []string{}
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		status := entry[:2]
		path := entry[3:]
		if status[0] == 'R' || status[0] == 'C' {
			i++
		}
		if strings.ContainsRune(status, 'D') {
			continue
		}
		out = append(out, path)
	}
	return out
}

func eachLine(r io.Reader, fn func(line string) bool) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 && !fn(line) {
			return
		}
		if err != nil {
			return
		}
	}
}

// SessionStart is the first transcript timestamp; ctime is never used because the transcript
// is appended right up to the Stop, which would make "now" the start and silence the gate.
func SessionStart(r io.Reader, now time.Time) time.Time {
	start := now.Add(-wideWindow)
	if r == nil {
		return start
	}
	scanned := 0
	eachLine(r, func(line string) bool {
		scanned++
		if scanned > maxTimestampScan {
			return false
		}
		var head struct {
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal([]byte(line), &head) != nil || head.Timestamp == "" {
			return true
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if ts, err := time.Parse(layout, head.Timestamp); err == nil {
				start = ts
				return false
			}
		}
		return true
	})
	return start
}

// SkillsLoaded counts a skill only on a real invocation: a Skill tool call or a read of its
// SKILL.md. Its name in the available-skills listing does not count.
func SkillsLoaded(r io.Reader) []string {
	found := []string{}
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			found = append(found, name)
		}
	}
	if r == nil {
		return found
	}
	eachLine(r, func(line string) bool {
		if m := skillCall.FindStringSubmatch(line); m != nil {
			add(m[1])
		}
		if m := skillRead.FindStringSubmatch(line); m != nil {
			add(m[1])
		}
		return true
	})
	return found
}

func BuildReason(files []string) string {
	shown := make([]string, 0, 6)
	for i, f := range files {
		if i == 6 {
			break
		}
		shown = append(shown, "  "+f)
	}
	list := strings.Join(shown, "\n")
	if len(files) > 6 {
		list += "\n  ... and " + itoa(len(files)-6) + " more"
	}
	return strings.Join([]string{
		"This session changed " + itoa(len(files)) + " production source file(s) without loading the adversarial testing discipline:",
		list,
		"",
		"Running the existing suite only asks whether the code does what you expect. Before reporting this",
		"done, invoke `test-strategy` (\"haz el testing\") so the change gets its contract classes, the",
		"inputs nobody expects, and a probe proven able to go red. If the change genuinely does not warrant",
		"it (generated code, a pure rename, a revert), say so in one line and finish.",
		"",
		"This is a reminder, not an approval gate. Silence it for this repository with a .no-testing-gate file.",
	}, "\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func isAdversarial(loaded []string) bool {
	for _, name := range loaded {
		for _, adv := range Adversarial {
			if name == adv {
				return true
			}
		}
	}
	return false
}

func openOrNil(d Deps, path string) io.ReadCloser {
	if path == "" || d.OpenTranscript == nil {
		return nil
	}
	rc, err := d.OpenTranscript(path)
	if err != nil {
		return nil
	}
	return rc
}

// Decide is the whole contract: fire when this session changed production source, no
// adversarial skill was invoked, the repository did not opt out, and the turn is not already
// continuing because of a previous Stop block.
func Decide(in Input, d Deps) Result {
	if in.StopHookActive {
		return Result{}
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = d.WorkDir
	}
	since := d.Now.Add(-wideWindow)
	if rc := openOrNil(d, in.TranscriptPath); rc != nil {
		since = SessionStart(rc, d.Now)
		rc.Close()
	}
	rootOut, err := d.Git(cwd, "rev-parse", "--show-toplevel")
	root := strings.TrimSpace(rootOut)
	if err != nil || root == "" {
		return Result{}
	}
	entry := &Entry{
		TS:           d.Now.UTC().Format(time.RFC3339),
		Session:      in.SessionID,
		Repo:         filepath.Base(root),
		SkillsLoaded: []string{},
	}
	rel, cfgErr := plan.ResolvePath(root, func(path string) (string, error) {
		return readPlan(d, path)
	})
	if cfgErr == nil {
		entry.Plan = rel
	}
	statusOut, err := d.Git(root, "status", "--porcelain", "-z", "-uall")
	if err != nil {
		entry.Skipped = "git_status_failed"
		return Result{Entry: entry}
	}
	entries := ParsePorcelain(statusOut)
	if len(entries) > MaxStatusEntries {
		entry.Skipped = "too_many_entries"
		return Result{Entry: entry}
	}
	files := []string{}
	seen := map[string]bool{}
	for _, p := range entries {
		if !IsProductionSource(p) || seen[p] {
			continue
		}
		info, err := d.Stat(filepath.Join(root, p))
		if err != nil {
			continue
		}
		if !info.ModTime().Before(since) {
			seen[p] = true
			files = append(files, p)
		}
	}
	entry.ChangedSource = len(files)
	if _, err := d.Stat(filepath.Join(root, ".no-testing-gate")); err == nil {
		entry.OptedOut = true
	}
	if len(files) > 0 && !entry.OptedOut {
		if rc := openOrNil(d, in.TranscriptPath); rc != nil {
			entry.SkillsLoaded = SkillsLoaded(rc)
			rc.Close()
		}
	}
	fire := len(files) > 0 && !entry.OptedOut && !isAdversarial(entry.SkillsLoaded)
	entry.Fired = fire
	res := Result{Fire: fire, Files: files, Entry: entry}
	if fire {
		res.Reason = BuildReason(files)
		return res
	}
	// The discipline ran. The second question is whether it ran all the way: a layer assigned
	// and never invoked leaves the report reading as coverage of a surface nobody examined.
	if len(files) > 0 && !entry.OptedOut && isAdversarial(entry.SkillsLoaded) {
		if cfgErr != nil {
			entry.Skipped = "plan_config_invalid"
			res.Problem = BuildDeclarationProblem(cfgErr)
			return res
		}
		planPath := filepath.Join(root, rel)
		if body, err := readPlan(d, planPath); err == nil {
			if gaps, err := plan.GapsIn(body); err == nil {
				res.Audit = true
				entry.Audited = true
				res.Owed, res.Pending = len(gaps.UnsweptLayers), len(gaps.PendingTargets)
				res.Unreadable, res.Unplanned = len(gaps.InterruptedTables), gaps.NoLayerMatrix
				if gaps.Any() {
					res.Reason = BuildAuditReason(rel, gaps.Report())
				} else {
					res.Reason = BuildCompleteReason(rel)
				}
			}
		}
	}
	return res
}

func readPlan(d Deps, path string) (string, error) {
	if d.ReadPlan != nil {
		return d.ReadPlan(path)
	}
	body, err := os.ReadFile(path)
	return string(body), err
}
