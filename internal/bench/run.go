package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Prompt is the whole instruction the agent receives: the skill must infer everything else.
const Prompt = "haz el testing"

// Options configures a benchmark run.
type Options struct {
	CasesGlob    string
	Model        string
	Runs         int
	MaxTurns     int
	Timeout      time.Duration
	SuiteTimeout time.Duration
	MaxCostUSD   float64 // 0 means no ceiling
	Out          string
	BenchDir     string
	SkillFile    string // for the history's skill_version
	DryRun       bool
	Keep         bool
	Agent        Agent // nil means the real claude CLI
	Log          io.Writer
}

// Agent runs the model once in a workspace; injected so tests never spawn claude.
type Agent func(ctx context.Context, ws string, opts Options) (AgentResult, error)

// Aggregate is the whole run's outcome.
type Aggregate struct {
	TS             string   `json:"ts"`
	Out            string   `json:"out"`
	Model          string   `json:"model"`
	DryRun         bool     `json:"dry_run"`
	Cases          []Result `json:"cases"`
	Defects        int      `json:"defects"`
	Found          int      `json:"found"`
	Recall         float64  `json:"recall"`
	FalsePositives int      `json:"false_positives"`
	CostUSD        float64  `json:"cost_usd"`
	Invalid        int      `json:"invalid"`
	CostCeilingHit bool     `json:"cost_ceiling_hit"`
}

// ExitCostCeiling is returned when the run stopped early because the cost ceiling was reached.
const ExitCostCeiling = 2

// Run scaffolds, runs, and scores every case; it returns the aggregate and a process exit code.
func Run(opts Options) (Aggregate, int) {
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	if opts.Runs <= 0 {
		opts.Runs = 1
	}
	if opts.Agent == nil {
		opts.Agent = claudeAgent
	}
	now := time.Now()
	agg := Aggregate{TS: now.UTC().Format(time.RFC3339), Out: opts.Out, Model: opts.Model, DryRun: opts.DryRun}
	caseDirs, err := listCases(resolveCasesGlob(opts.BenchDir, opts.CasesGlob))
	if err != nil || len(caseDirs) == 0 {
		fmt.Fprintf(opts.Log, "no cases match %q\n", opts.CasesGlob)
		return agg, 1
	}
	if err := os.MkdirAll(opts.Out, 0o755); err != nil {
		fmt.Fprintln(opts.Log, "out:", err)
		return agg, 1
	}
	code := 0
loop:
	for _, caseDir := range caseDirs {
		key, err := LoadKey(caseDir)
		name := filepath.Base(caseDir)
		if err != nil {
			fmt.Fprintf(opts.Log, "[%s] skipped: %v\n", name, err)
			agg.Cases = append(agg.Cases, Result{Case: name, Invalid: true, InvalidReason: err.Error()})
			agg.Invalid++
			continue
		}
		for run := 1; run <= opts.Runs; run++ {
			res := runOnce(caseDir, key, run, opts)
			agg.Cases = append(agg.Cases, res)
			agg.CostUSD += res.CostUSD
			if res.Invalid {
				agg.Invalid++
			} else {
				agg.Defects += res.Total
				agg.Found += res.Found
				agg.FalsePositives += res.FalsePositives
			}
			fmt.Fprintf(opts.Log, "[%s #%d] recall %.2f (%d/%d) fp %d cost $%.3f turns %d%s\n",
				name, run, res.Recall, res.Found, res.Total, res.FalsePositives, res.CostUSD, res.Turns, invalidTag(res))
			if opts.MaxCostUSD > 0 && agg.CostUSD >= opts.MaxCostUSD {
				agg.CostCeilingHit = true
				code = ExitCostCeiling
				fmt.Fprintf(opts.Log, "cost ceiling $%.2f reached; stopping\n", opts.MaxCostUSD)
				break loop
			}
		}
	}
	if agg.Defects > 0 {
		agg.Recall = float64(agg.Found) / float64(agg.Defects)
	}
	writeJSON(filepath.Join(opts.Out, "aggregate.json"), agg)
	_ = os.WriteFile(filepath.Join(opts.Out, "summary.md"), []byte(Summary(agg)), 0o644)
	if !opts.DryRun && opts.BenchDir != "" {
		_ = AppendHistory(opts.BenchDir, HistoryEntry{
			TS: agg.TS, Out: opts.Out, Model: opts.Model, Cases: len(caseDirs), Defects: agg.Defects,
			Found: agg.Found, Recall: agg.Recall, FalsePositives: agg.FalsePositives, CostUSD: agg.CostUSD,
			SkillVersion: SkillVersion(opts.SkillFile),
		})
	}
	return agg, code
}

func runOnce(caseDir string, key Key, run int, opts Options) Result {
	name := filepath.Base(caseDir)
	ws := filepath.Join(opts.Out, name, fmt.Sprint(run), "ws")
	res := Result{Case: name, Run: run, Total: len(key.Defects), Workspace: ws}
	suite, err := Scaffold(caseDir, ws, key, opts.SuiteTimeout)
	res.Suite = suite
	if err != nil {
		res.Invalid, res.InvalidReason = true, err.Error()
		return finish(res, opts, true)
	}
	if !suite.Green {
		res.Invalid, res.InvalidReason = true, fmt.Sprintf("fixture suite not green (exit %d)", suite.ExitCode)
		return finish(res, opts, true)
	}
	if opts.DryRun {
		res.Notes = append(res.Notes, "dry-run: agent not spawned")
		scored := ScoreWorkspace(ws, key)
		res = merge(res, scored)
		return finish(res, opts, false)
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	started := time.Now()
	ar, err := opts.Agent(ctx, ws, opts)
	res.Seconds = time.Since(started).Seconds()
	res.CostUSD, res.Turns = ar.CostUSD, ar.Turns
	if ar.TimedOut {
		res.Notes = append(res.Notes, "agent timed out")
	}
	if err != nil {
		res.Notes = append(res.Notes, "agent error: "+err.Error())
	}
	res = merge(res, ScoreWorkspace(ws, key))
	failed := err != nil || ar.TimedOut
	return finish(res, opts, failed)
}

// merge keeps the run's bookkeeping fields and takes the scorer's fields.
func merge(res, scored Result) Result {
	scored.Case, scored.Run, scored.Workspace = res.Case, res.Run, res.Workspace
	scored.CostUSD, scored.Turns, scored.Seconds = res.CostUSD, res.Turns, res.Seconds
	scored.Suite, scored.Invalid, scored.InvalidReason = res.Suite, res.Invalid, res.InvalidReason
	scored.Notes = append(res.Notes, scored.Notes...)
	return scored
}

func finish(res Result, opts Options, keepWS bool) Result {
	dir := filepath.Dir(res.Workspace)
	writeJSON(filepath.Join(dir, "result.json"), res)
	if !opts.Keep && !keepWS && !res.Invalid {
		_ = os.RemoveAll(res.Workspace)
		res.Workspace = ""
	}
	return res
}

func invalidTag(r Result) string {
	if r.Invalid {
		return " INVALID: " + r.InvalidReason
	}
	return ""
}

// A pattern without a path separator names cases under <benchDir>/cases, so `n0*` works from
// anywhere; a pattern with a separator is used as given.
func resolveCasesGlob(benchDir, glob string) string {
	if !strings.ContainsAny(glob, `/\`) {
		return filepath.Join(benchDir, "cases", glob)
	}
	return glob
}

func listCases(glob string) ([]string, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && st.IsDir() {
			if _, err := os.Stat(filepath.Join(m, KeyFile)); err == nil {
				dirs = append(dirs, m)
			}
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func writeJSON(path string, v any) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}

// Summary renders the aggregate as the markdown table written to summary.md.
func Summary(agg Aggregate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Bench %s\n\nModel: %s · cases: %d · defects: %d · found: %d · recall: %.2f · false positives: %d · cost: $%.3f",
		agg.TS, agg.Model, len(agg.Cases), agg.Defects, agg.Found, agg.Recall, agg.FalsePositives, agg.CostUSD)
	if agg.DryRun {
		b.WriteString(" · dry-run")
	}
	if agg.CostCeilingHit {
		b.WriteString(" · cost ceiling hit")
	}
	b.WriteString("\n\n| case | recall | found/total | false positives | cost USD | turns | minutes | invalid? |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range agg.Cases {
		inv := ""
		if c.Invalid {
			inv = c.InvalidReason
		}
		fmt.Fprintf(&b, "| %s | %.2f | %d/%d | %d | %.3f | %d | %.1f | %s |\n",
			c.Case, c.Recall, c.Found, c.Total, c.FalsePositives, c.CostUSD, c.Turns, c.Seconds/60, inv)
	}
	return b.String()
}

// claudeAgent spawns the real CLI with the single prompt and reads its stream-json.
func claudeAgent(ctx context.Context, ws string, opts Options) (AgentResult, error) {
	args := []string{"-p", Prompt, "--output-format", "stream-json", "--verbose",
		"--max-turns", fmt.Sprint(opts.MaxTurns), "--permission-mode", "bypassPermissions"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = ws
	cmd.Stdin = strings.NewReader("")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	ar := ParseStream(&out)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		ar.TimedOut = true
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			ar.ExitCode = ee.ExitCode()
		}
		if !ar.TimedOut {
			return ar, fmt.Errorf("claude: %v: %s", err, tail(errb.String(), 500))
		}
	}
	return ar, nil
}
