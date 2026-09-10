package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

// HistoryEntry is one benchmark run as remembered across skill versions.
type HistoryEntry struct {
	TS             string  `json:"ts"`
	Out            string  `json:"out"`
	Model          string  `json:"model"`
	Cases          int     `json:"cases"`
	Defects        int     `json:"defects"`
	Found          int     `json:"found"`
	Recall         float64 `json:"recall"`
	FalsePositives int     `json:"false_positives"`
	CostUSD        float64 `json:"cost_usd"`
	SkillVersion   string  `json:"skill_version"`
	Failed         int     `json:"failed"`
	Invalid        int     `json:"invalid"`
	NoPlan         int     `json:"no_plan"`
	Caught         int     `json:"caught"`
	RecallCaught   float64 `json:"recall_caught"`
	Kind           string  `json:"kind"`                 // KindRun or KindRescore
	RunTS          string  `json:"run_ts,omitempty"`     // rescore: when the run it re-reads happened
	SourceRun      string  `json:"source_run,omitempty"` // rescore: the results directory it re-read
	Scorer         string  `json:"scorer"`               // build that produced the numbers, so two readings of one run are ordered
}

// A row is either a run that spawned agents or a rescore that re-read one with newer rules.
const (
	KindRun     = "run"
	KindRescore = "rescore"
)

const historyHeader = "| ts | kind | out | model | cases | defects | reported | recall | caught | recall caught | false positives | failed | invalid | no plan | cost USD | skill version | scorer |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n"

// reported and caught count different things, so neither bounds the other.
const historyIntro = "# Benchmark history\n\nOne row per `rdd-plus bench run`; never rewritten. A `rescore` row re-reads an earlier\nrun with newer scoring rules: it spends nothing, so summing the cost column over rescore\nrows would count the same money twice. `reported and caught are independent`: reported\ncounts defects written in the plan, caught counts defects some test distinguishes, and\neither can exceed the other.\n\nEvery row names the `scorer` build that produced its numbers. When the scoring rules change,\na later rescore of one source run supersedes an earlier one, and the scorer column is what\ntells the two apart; rows are never rewritten.\n\n"

// AppendHistory adds one line to history.jsonl and one row to history.md under benchDir;
// both files are append-only and never rewritten.
func AppendHistory(benchDir string, e HistoryEntry) error {
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		return err
	}
	if e.Kind == "" {
		e.Kind = KindRun
	}
	if e.Scorer == "" {
		e.Scorer = scorerRevision()
	}
	if e.Kind == KindRescore {
		e.CostUSD = 0 // the run it re-reads already carries that cost
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := appendFile(filepath.Join(benchDir, "history.jsonl"), append(line, '\n')); err != nil {
		return err
	}
	md := filepath.Join(benchDir, "history.md")
	existing, err := os.ReadFile(md)
	if os.IsNotExist(err) {
		if err := appendFile(md, []byte(historyIntro+historyHeader)); err != nil {
			return err
		}
	} else if !strings.Contains(string(existing), historyHeader) {
		// The columns changed: rows already written stay as they are under their own header.
		if err := appendFile(md, []byte("\n"+historyHeader)); err != nil {
			return err
		}
	}
	ts, kind := e.TS, e.Kind
	if e.Kind == KindRescore {
		if e.RunTS != "" {
			ts = e.RunTS // the row describes that run, not the moment it was re-read
		}
		kind = "rescore of " + e.SourceRun
	}
	row := fmt.Sprintf("| %s | %s | %s | %s | %d | %d | %d | %.2f | %d | %.2f | %d | %d | %d | %d | %.3f | %s | %s |\n",
		ts, kind, e.Out, e.Model, e.Cases, e.Defects, e.Found, e.Recall, e.Caught, e.RecallCaught, e.FalsePositives, e.Failed, e.Invalid, e.NoPlan, e.CostUSD, e.SkillVersion, e.Scorer)
	return appendFile(md, []byte(row))
}

func appendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

var versionRe = regexp.MustCompile(`(?m)^\s*version:\s*"?([^"\n]+)"?\s*$`)

// SkillVersion reads the version from a skill's frontmatter, or "unknown".
func SkillVersion(skillFile string) string {
	raw, err := os.ReadFile(skillFile)
	if err != nil {
		return "unknown"
	}
	if m := versionRe.FindSubmatch(raw); m != nil {
		return string(m[1])
	}
	return "unknown"
}

// Stamp is the results directory name for a run started at t.
func Stamp(t time.Time) string {
	return t.UTC().Format("20060102-150405")
}

// scorerRevision identifies the build whose rules produced a row. Go embeds the commit when the
// binary is built inside a repository; a build without that information still names itself, so
// a row is never unattributable.
func scorerRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "unknown"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		return rev + "+dirty"
	}
	return rev
}

// ScorerIsProvisional reports whether a scorer revision fails to identify the code that produced
// the numbers: a build from an uncommitted tree, or one with no version control information.
func ScorerIsProvisional(rev string) bool {
	return rev == "" || rev == "unknown" || strings.HasSuffix(rev, "+dirty")
}

// ProvisionalScorerWarning is what a run says before spending anything.
func ProvisionalScorerWarning(rev string) string {
	return "scorer " + rev + ": this build is not reproducible from a commit, so the numbers it " +
		"records cannot be re-derived later. Commit before measuring anything you intend to cite."
}
