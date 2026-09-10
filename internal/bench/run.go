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
	Retries      int           // agent retries on infrastructure failures (exit status, error result)
	RetryDelay   time.Duration // pause before a retry, so a rate limit has time to lift
	Agent        Agent         // nil means the real claude CLI
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
	Caught         int      `json:"caught"`        // defects distinguished by an agent test
	RecallCaught   float64  `json:"recall_caught"` // caught over defects of valid runs
	FalsePositives int      `json:"false_positives"`
	CostUSD        float64  `json:"cost_usd"`
	Invalid        int      `json:"invalid"`
	Failed         int      `json:"failed"`  // agent did not run to completion; excluded from recall
	NoPlan         int      `json:"no_plan"` // valid runs that never wrote docs/testing/test-plan.md; scored zero
	CostCeilingHit bool     `json:"cost_ceiling_hit"`
}

// ExitCostCeiling is returned when the run stopped early because the cost ceiling was reached.
const ExitCostCeiling = 2

// ExitPartial is returned when some case failed or was invalid: the numbers are incomplete
// evidence and must not be read as a measurement of the whole corpus.
const ExitPartial = 3

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
	var patterns []string
	for _, g := range strings.Split(opts.CasesGlob, ",") {
		patterns = append(patterns, resolveCasesGlob(opts.BenchDir, strings.TrimSpace(g)))
	}
	caseDirs, err := listCases(strings.Join(patterns, ","))
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
			} else if res.Failed {
				agg.Failed++
			} else {
				agg.Defects += res.Total
				agg.Found += res.Found
				agg.Caught += res.Caught
				agg.FalsePositives += res.FalsePositives
				if !res.PlanFound {
					agg.NoPlan++
				}
			}
			fmt.Fprintf(opts.Log, "[%s #%d] reported %d/%d caught %d/%d fp %d cost $%.3f turns %d%s%s\n",
				name, run, res.Found, res.Total, res.Caught, res.Total, res.FalsePositives, res.CostUSD, res.Turns, invalidTag(res), noPlanTag(res))
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
		agg.RecallCaught = float64(agg.Caught) / float64(agg.Defects)
	}
	if code == 0 && (agg.Failed > 0 || agg.Invalid > 0) {
		code = ExitPartial
		fmt.Fprintf(opts.Log, "partial: %d failed, %d invalid; recall covers valid cases only\n", agg.Failed, agg.Invalid)
	}
	writeJSON(filepath.Join(opts.Out, "aggregate.json"), agg)
	_ = os.WriteFile(filepath.Join(opts.Out, "summary.md"), []byte(Summary(agg)), 0o644)
	if !opts.DryRun && opts.BenchDir != "" {
		_ = AppendHistory(opts.BenchDir, HistoryEntry{
			TS: agg.TS, Out: opts.Out, Model: opts.Model, Cases: len(caseDirs), Defects: agg.Defects,
			Found: agg.Found, Recall: agg.Recall, Caught: agg.Caught, RecallCaught: agg.RecallCaught,
			FalsePositives: agg.FalsePositives, CostUSD: agg.CostUSD,
			Failed: agg.Failed, Invalid: agg.Invalid, NoPlan: agg.NoPlan,
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
	started := time.Now()
	var ar AgentResult
	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		ar, err = opts.Agent(ctx, ws, opts)
		cancel()
		res.CostUSD += ar.CostUSD
		res.Turns += ar.Turns
		writeAgentLog(filepath.Dir(ws), attempt, ar, err)
		infra := err != nil || ar.IsError
		if !infra || ar.TimedOut || attempt >= opts.Retries {
			break
		}
		res.Notes = append(res.Notes, fmt.Sprintf("attempt %d failed, retrying after %s", attempt+1, opts.RetryDelay))
		time.Sleep(opts.RetryDelay)
	}
	res.Seconds = time.Since(started).Seconds()
	switch {
	case ar.TimedOut:
		res.Failed, res.FailReason = true, "agent timed out"
	case err != nil:
		res.Failed, res.FailReason = true, "agent error: "+err.Error()
	case ar.IsError:
		res.Failed, res.FailReason = true, "agent result is an error: "+tail(ar.ErrorText, 200)
	}
	if res.Failed {
		// An agent that never ran is not a detection result; scoring it would read as recall 0.
		res.Notes = append(res.Notes, res.FailReason)
		return finish(res, opts, true)
	}
	res = merge(res, ScoreWorkspace(ws, key))
	res.Catch = Discriminate(caseDir, ws, key, opts.SuiteTimeout)
	res.Caught = res.Catch.Count()
	// Misses of either measure keep their workspace so they can be classified.
	return finish(res, opts, res.Recall < 1 || (res.Catch.Checked && res.Caught < res.Total))
}

// writeAgentLog keeps the tail of each attempt's raw stream beside result.json.
func writeAgentLog(dir string, attempt int, ar AgentResult, err error) {
	var b strings.Builder
	fmt.Fprintf(&b, "attempt %d exit_code=%d is_error=%v timed_out=%v err=%v\n", attempt+1, ar.ExitCode, ar.IsError, ar.TimedOut, err)
	b.WriteString(ar.Raw)
	b.WriteString("\n")
	f, e := os.OpenFile(filepath.Join(dir, "agent.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if e == nil {
		_, _ = f.WriteString(b.String())
		_ = f.Close()
	}
}

// merge keeps the run's bookkeeping fields and takes the scorer's fields.
func merge(res, scored Result) Result {
	scored.Case, scored.Run, scored.Workspace = res.Case, res.Run, res.Workspace
	scored.CostUSD, scored.Turns, scored.Seconds = res.CostUSD, res.Turns, res.Seconds
	scored.Suite, scored.Invalid, scored.InvalidReason = res.Suite, res.Invalid, res.InvalidReason
	scored.Failed, scored.FailReason = res.Failed, res.FailReason
	scored.Notes = append(res.Notes, scored.Notes...)
	return scored
}

func finish(res Result, opts Options, keepWS bool) Result {
	dir := filepath.Dir(res.Workspace)
	writeJSON(filepath.Join(dir, "result.json"), res)
	// The plan is the run's deliverable: keep it beside result.json so a later scorer can re-read it.
	if data, err := os.ReadFile(filepath.Join(res.Workspace, PlanPath)); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "test-plan.md"), data, 0o644)
	}
	if !opts.Keep && !keepWS && !res.Invalid && !res.Failed {
		_ = os.RemoveAll(res.Workspace)
		res.Workspace = ""
	}
	return res
}

// A valid run that wrote no plan scores zero but is reported apart: the flow ran and never
// persisted its deliverable, which is a different failure from missing the defect.
func noPlanTag(r Result) string {
	if !r.Invalid && !r.Failed && !r.PlanFound {
		return " NO PLAN"
	}
	return ""
}

func invalidTag(r Result) string {
	if r.Invalid {
		return " INVALID: " + r.InvalidReason
	}
	if r.Failed {
		return " FAILED: " + r.FailReason
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
	var matches []string
	for _, g := range strings.Split(glob, ",") {
		m, err := filepath.Glob(strings.TrimSpace(g))
		if err != nil {
			return nil, err
		}
		matches = append(matches, m...)
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
	fmt.Fprintf(&b, "# Bench %s\n\nModel: %s · cases: %d · defects: %d · reported: %d (%.2f) · caught by a test: %d (%.2f) · false positives: %d · failed: %d · invalid: %d · no plan: %d · cost: $%.3f",
		agg.TS, agg.Model, len(agg.Cases), agg.Defects, agg.Found, agg.Recall, agg.Caught, agg.RecallCaught, agg.FalsePositives, agg.Failed, agg.Invalid, agg.NoPlan, agg.CostUSD)
	if agg.DryRun {
		b.WriteString(" · dry-run")
	}
	if agg.CostCeilingHit {
		b.WriteString(" · cost ceiling hit")
	}
	b.WriteString("\n\n| case | reported | caught | false positives | cost USD | turns | minutes | note |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range agg.Cases {
		note := strings.TrimSpace(invalidTag(c) + noPlanTag(c))
		if c.Catch.Checked && len(c.Catch.Notes) > 0 {
			note = strings.TrimSpace(note + " " + strings.Join(c.Catch.Notes, "; "))
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %d/%d | %d | %.3f | %d | %.1f | %s |\n",
			c.Case, c.Found, c.Total, c.Caught, c.Total, c.FalsePositives, c.CostUSD, c.Turns, c.Seconds/60, note)
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
	ar := ParseStream(bytes.NewReader(out.Bytes()))
	ar.Raw = tail(out.String(), 64*1024) + "\n--- stderr ---\n" + tail(errb.String(), 4*1024)
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
