package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
}

const historyHeader = "| ts | out | model | cases | defects | found | recall | false positives | cost USD | skill version |\n|---|---|---|---|---|---|---|---|---|---|\n"

// AppendHistory adds one line to history.jsonl and one row to history.md under benchDir;
// both files are append-only and never rewritten.
func AppendHistory(benchDir string, e HistoryEntry) error {
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := appendFile(filepath.Join(benchDir, "history.jsonl"), append(line, '\n')); err != nil {
		return err
	}
	md := filepath.Join(benchDir, "history.md")
	if _, err := os.Stat(md); os.IsNotExist(err) {
		if err := appendFile(md, []byte("# Benchmark history\n\nOne row per `rdd-plus bench run`; never rewritten.\n\n"+historyHeader)); err != nil {
			return err
		}
	}
	row := fmt.Sprintf("| %s | %s | %s | %d | %d | %d | %.2f | %d | %.3f | %s |\n",
		e.TS, e.Out, e.Model, e.Cases, e.Defects, e.Found, e.Recall, e.FalsePositives, e.CostUSD, e.SkillVersion)
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
