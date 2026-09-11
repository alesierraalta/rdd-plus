// rdd-plus is a deterministic companion for the testing discipline: it installs the skills,
// wires the Stop hook that keeps them invoked, and reports what the environment can do.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/bench"
	"github.com/alesierraalta/rdd-plus/internal/buildinfo"
	"github.com/alesierraalta/rdd-plus/internal/check"
	"github.com/alesierraalta/rdd-plus/internal/doctor"
	"github.com/alesierraalta/rdd-plus/internal/feedback"
	"github.com/alesierraalta/rdd-plus/internal/gate"
	"github.com/alesierraalta/rdd-plus/internal/plan"
	"github.com/alesierraalta/rdd-plus/internal/sync"
)

const usage = `usage: rdd-plus <command> [flags]

commands:
  gate     Stop hook: read the hook payload on stdin, decide, log, emit feedback
  sync     install the embedded skills and wire the gate into settings.json
  doctor   report installed skills, the hook wiring, and optional capabilities
  bench    run the testing skill against sealed-key fixtures and score it (run | score | history)
  plan     write the skeleton, check the contract, and name what breadth is still owed
           (init | check | gaps)
  check    say what this repository still owes, from git and the plan alone: no hook payload,
           no transcript, no host. Exit 1 when there is something to do.
  feedback record an honest process report on the method itself, or read the reports back
           (--template | --file <path> | --summary)
  version  print the version

flags shared by gate, sync, doctor, feedback:
  --config-dir <dir>   Claude config directory (default: ~/.claude)

bench run [--cases <glob>] [--runner pi|claude] [--model <m>] [--runs N] [--max-turns N] [--timeout 30m]
          [--max-cost-usd N] [--out <dir>] [--bench-dir <dir>] [--dry-run] [--keep]
          [--retries N] [--retry-delay 60s] [--agent-config bench|<dir>] [--concurrency N]
          (--cases accepts comma-separated patterns)
          (--model is a name for claude and <provider>/<model>[:<thinking>] for pi; --runner pi
           builds a throwaway agent dir under --out unless --agent-config names one)
bench score --case <dir> --workspace <ws>
bench history [--bench-dir <dir>]
bench compare <before-results> <after-results>
bench rescore [--bench-dir <dir>] <results>
plan init [--path docs/testing/test-plan.md] [--force]
plan check [--path docs/testing/test-plan.md]
plan gaps  [--path docs/testing/test-plan.md]
           (swept = status done, fixed or closed; n/a, na, none and skipped leave the denominator)
check [--cwd .]
feedback [--config-dir <dir>] [--template] [--file <path>] [--plan <path>] [--summary]
`

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gate":
		os.Exit(runGate(os.Args[2:]))
	case "sync":
		os.Exit(runSync(os.Args[2:]))
	case "doctor":
		os.Exit(runDoctor(os.Args[2:]))
	case "bench":
		os.Exit(runBench(os.Args[2:]))
	case "plan":
		os.Exit(runPlan(os.Args[2:]))
	case "check":
		os.Exit(runCheck(os.Args[2:]))
	case "feedback":
		os.Exit(runFeedback(os.Args[2:]))
	case "version":
		fmt.Println(buildinfo.String())
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func runGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "Claude config directory")
	if err := fs.Parse(args); err != nil {
		return 0 // a hook must never break the turn, even on a bad flag
	}
	return gate.Run(os.Stdin, os.Stdout, gate.DefaultLogPath(*configDir), time.Now())
}

func runSync(args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	configDir := fs.String("config-dir", defaultConfigDir(), "Claude config directory")
	dryRun := fs.Bool("dry-run", false, "print the plan and write nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	bin, err := os.Executable()
	if err == nil {
		bin, _ = filepath.Abs(bin)
	}
	report, err := sync.Sync(*configDir, bin, sync.Options{DryRun: *dryRun})
	fmt.Print(report.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return 1
	}
	return 0
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configDir := fs.String("config-dir", defaultConfigDir(), "Claude config directory")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	report := doctor.RunWith(*configDir, exec.LookPath, probeHook)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Print(report.String())
	}
	if report.Healthy {
		return 0
	}
	return 1
}

// selfDir is the directory of the running binary, so a spawned agent runs this build when the
// skill tells it to call `rdd-plus plan init`.
func selfDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	return filepath.Dir(exe)
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	cwd := fs.String("cwd", ".", "directory inside the repository to check")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	res := check.Run(*cwd, check.Deps{})
	fmt.Println(strings.TrimRight(res.Text, "\n"))
	return res.Exit
}

// runFeedback is the destination the gate's Stop offer never had: --template hands the operator a
// fillable report, --file records it, and no flags reads the reports back.
func runFeedback(args []string) int {
	fs := flag.NewFlagSet("feedback", flag.ContinueOnError)
	configDir := fs.String("config-dir", defaultConfigDir(), "Claude config directory")
	template := fs.Bool("template", false, "print a fillable skeleton and write nothing")
	file := fs.String("file", "", "submit the report written in this file")
	plan := fs.String("plan", "", "repository-relative plan path the report is about")
	summary := fs.Bool("summary", false, "read the reports back and print the summary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch {
	case *template:
		fmt.Print(feedback.Template(feedback.Report{
			TS:    time.Now().UTC().Format(time.RFC3339),
			Repo:  feedback.RepoRoot("."),
			Plan:  *plan,
			Skill: feedback.EmbeddedSkillVersion(),
			Build: buildinfo.String(),
		}))
		return 0
	case *file != "":
		raw, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "feedback:", err)
			return 1
		}
		r, err := feedback.Parse(string(raw))
		if err != nil {
			fmt.Fprintln(os.Stderr, "feedback:", err)
			return 2
		}
		if *plan != "" {
			r.Plan = *plan
		}
		if strings.TrimSpace(r.Plan) == "" {
			r.Plan = feedback.NotGiven
		}
		if err := feedback.Record(*configDir, r); err != nil {
			fmt.Fprintln(os.Stderr, "feedback:", err)
			return 1
		}
		fmt.Printf("recorded %s feedback for %s\n", r.Verdict, r.Repo)
		return 0
	default:
		_ = *summary // --summary and no flags are the same cheapest path to the answer
		out, err := feedback.Summary(*configDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "feedback:", err)
			return 1
		}
		fmt.Print(out)
		return 0
	}
}

func runPlan(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	path := fs.String("path", plan.DefaultPath, "plan file")
	force := fs.Bool("force", false, "replace an existing plan (init only)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	switch args[0] {
	case "gaps":
		g, err := plan.GapsInFile(*path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "plan gaps:", err)
			return 1
		}
		fmt.Print(g.Report())
		if g.Any() {
			return 1
		}
		return 0
	case "init":
		if err := plan.Init(*path, *force); err != nil {
			fmt.Fprintln(os.Stderr, "plan init:", err)
			return 1
		}
		fmt.Printf("wrote %s\n", *path)
		return 0
	case "check":
		problems, err := plan.Check(*path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "plan check:", err)
			return 1
		}
		if len(problems) == 0 {
			fmt.Printf("%s: well formed\n", *path)
			return 0
		}
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "%s: %s\n", *path, p)
		}
		return 1
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// probeHook runs the wired Stop command the way Claude Code does, with an empty payload on
// stdin, and requires it to exit zero. Matching the command string proves nothing: a binary that
// needs a subcommand looks identical to one that does not.
func probeHook(command string) error {
	fields, err := shellFields(command)
	if err != nil || len(fields) == 0 {
		return fmt.Errorf("cannot read the wired command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.Stdin = strings.NewReader("{}")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run()
}

// shellFields splits a hook command on spaces, honouring the double quotes a path with spaces
// needs. It is not a shell: a command that needs one is beyond what this probe can check.
func shellFields(command string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for _, r := range command {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unbalanced quote")
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

func runBench(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "run":
		return runBenchRun(args[1:])
	case "score":
		return runBenchScore(args[1:])
	case "history":
		return runBenchHistory(args[1:])
	case "compare":
		return runBenchCompare(args[1:])
	case "rescore":
		return runBenchRescore(args[1:])
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

func runBenchRun(args []string) int {
	fs := flag.NewFlagSet("bench run", flag.ContinueOnError)
	cases := fs.String("cases", "bench/cases/*", "glob of case directories (each with fixture/ and KEY.json)")
	runner := fs.String("runner", bench.RunnerPi, "agent runner: pi (default) or claude (last resort)")
	model := fs.String("model", "", "model for the agent runs; empty uses the runner's default ("+
		bench.DefaultModel(bench.RunnerPi)+" for pi, "+bench.DefaultModel(bench.RunnerClaude)+" for claude)")
	runs := fs.Int("runs", 1, "runs per case")
	maxTurns := fs.Int("max-turns", 70, "agent turn cap per run")
	timeout := fs.Duration("timeout", 30*time.Minute, "agent wall-clock cap per run")
	suiteTimeout := fs.Duration("suite-timeout", 10*time.Minute, "fixture suite cap")
	maxCost := fs.Float64("max-cost-usd", 0, "stop when the cumulative cost reaches this (0 = no ceiling)")
	out := fs.String("out", "", "results directory (default: <bench-dir>/results/<timestamp>)")
	benchDir := fs.String("bench-dir", "bench", "benchmark directory holding history.jsonl and history.md")
	dryRun := fs.Bool("dry-run", false, "scaffold and check fixtures, spawn no agent, write no history")
	keep := fs.Bool("keep", false, "keep workspaces after scoring")
	retries := fs.Int("retries", 1, "retries per case on infrastructure failures (exit status, error result)")
	retryDelay := fs.Duration("retry-delay", 60*time.Second, "pause before a retry")
	workers := fs.Int("concurrency", 1, "cases to run side by side; the wall clock shortens, the cost does not")
	agentConfig := fs.String("agent-config", "", "agent config directory; \"bench\" builds a throwaway one holding only the embedded skills (the default for --runner pi)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !bench.KnownRunner(*runner) {
		fmt.Fprintf(os.Stderr, "bench run: unknown runner %q: use %s or %s\n", *runner, bench.RunnerPi, bench.RunnerClaude)
		return 2
	}
	if *model == "" {
		*model = bench.DefaultModel(*runner)
	}
	if *out == "" {
		*out = filepath.Join(*benchDir, "results", bench.Stamp(time.Now()))
	}
	cfgDir, skillFile := *agentConfig, filepath.Join(defaultConfigDir(), "skills", "test-strategy", "SKILL.md")
	switch {
	case *runner == bench.RunnerPi && (cfgDir == "" || cfgDir == "bench"):
		// A Pi run always gets a throwaway config: the runner exists so a reading is not shaped by
		// the operator's packages, extensions, memory protocol, or MCP servers, and so no credential
		// is shared by link. It is built under the results directory, which the run owns and git
		// ignores, rather than beside the Claude one in bench/.
		cfgDir = filepath.Join(*out, ".pi-agent-config")
		from := bench.DefaultPiConfigDir()
		if err := bench.WriteBenchPiConfig(cfgDir, from); err != nil {
			fmt.Fprintln(os.Stderr, "bench config:", err)
			return 1
		}
		if _, err := os.Stat(filepath.Join(from, bench.PiAuthFile)); err != nil {
			// Nothing to copy and nothing to link: the run stays alive and reports the authentication
			// failure itself, with the CLI's own message, instead of failing here for a wrong reason.
			fmt.Fprintf(os.Stderr, "bench config: no %s in %s; the run will report the authentication error itself\n", bench.PiAuthFile, from)
		}
		if abs, err := filepath.Abs(cfgDir); err == nil {
			cfgDir = abs
		}
	case cfgDir == "bench":
		// A throwaway configuration holding only the embedded skills, so the run measures them
		// and not the operator's global instructions, memory protocol, or MCP servers.
		cfgDir = filepath.Join(*benchDir, ".agent-config")
		if err := bench.WriteBenchConfig(cfgDir, defaultConfigDir()); err != nil {
			fmt.Fprintln(os.Stderr, "bench config:", err)
			return 1
		}
		if abs, err := filepath.Abs(cfgDir); err == nil {
			cfgDir = abs
		}
	}
	if cfgDir != "" {
		skillFile = filepath.Join(cfgDir, "skills", "test-strategy", "SKILL.md")
	}
	_, code := bench.Run(bench.Options{
		CasesGlob: *cases, Model: *model, Runner: *runner, Runs: *runs, MaxTurns: *maxTurns,
		Timeout:      *timeout,
		SuiteTimeout: *suiteTimeout, MaxCostUSD: *maxCost, Out: *out, BenchDir: *benchDir,
		SkillFile: skillFile,
		ConfigDir: cfgDir, BinDir: selfDir(), Workers: *workers,
		DryRun: *dryRun, Keep: *keep, Retries: *retries, RetryDelay: *retryDelay, Log: os.Stdout,
	})
	fmt.Printf("results: %s\n", filepath.Join(*out, "summary.md"))
	return code
}

func runBenchScore(args []string) int {
	fs := flag.NewFlagSet("bench score", flag.ContinueOnError)
	caseDir := fs.String("case", "", "case directory holding KEY.json")
	ws := fs.String("workspace", "", "workspace to score")
	planFile := fs.String("plan", "", "plan file to score, such as the test-plan.md a run keeps beside result.json")
	if err := fs.Parse(args); err != nil || *caseDir == "" || (*ws == "") == (*planFile == "") {
		fmt.Fprintln(os.Stderr, "bench score needs --case and exactly one of --workspace or --plan")
		return 2
	}
	key, err := bench.LoadKey(*caseDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "score:", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if *planFile != "" {
		_ = enc.Encode(bench.ScorePlanFile(*planFile, key))
		return 0
	}
	res := bench.ScoreWorkspace(*ws, key)
	res.Catch = bench.Discriminate(*caseDir, *ws, key, 10*time.Minute)
	res.Caught = res.Catch.Count()
	_ = enc.Encode(res)
	return 0
}

func runBenchRescore(args []string) int {
	fs := flag.NewFlagSet("bench rescore", flag.ContinueOnError)
	benchDir := fs.String("bench-dir", "bench", "benchmark directory holding cases/ and the history")
	suiteTimeout := fs.Duration("suite-timeout", 10*time.Minute, "cap per suite run")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "bench rescore needs one results directory")
		return 2
	}
	results := fs.Arg(0)
	lookup := func(name string) (string, error) {
		dir := filepath.Join(*benchDir, "cases", name)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return "", fmt.Errorf("no case directory %s", dir)
		}
		return dir, nil
	}
	agg, err := bench.Rescore(results, lookup, *suiteTimeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rescore:", err)
		return 1
	}
	_ = bench.AppendHistory(*benchDir, bench.HistoryEntry{
		TS: agg.TS, Out: agg.Out, Model: agg.Model, Cases: len(agg.Cases), Defects: agg.Defects,
		Found: agg.Found, Recall: agg.Recall, Caught: agg.Caught, RecallCaught: agg.RecallCaught,
		FalsePositives: agg.FalsePositives, Failed: agg.Failed, Invalid: agg.Invalid, NoPlan: agg.NoPlan,
		Kind: bench.KindRescore, RunTS: agg.RunTS, SourceRun: results, SkillVersion: agg.SkillVersion,
		Corpus: agg.Corpus,
	})
	fmt.Print(bench.Summary(agg))
	return 0
}

func runBenchCompare(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "bench compare needs two result directories: <before> <after>")
		return 2
	}
	cmp, err := bench.Compare(args[0], args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "compare:", err)
		return 1
	}
	fmt.Print(cmp.Markdown())
	return 0
}

func runBenchHistory(args []string) int {
	fs := flag.NewFlagSet("bench history", flag.ContinueOnError)
	benchDir := fs.String("bench-dir", "bench", "benchmark directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	data, err := os.ReadFile(filepath.Join(*benchDir, "history.md"))
	if err != nil {
		fmt.Println("no benchmark history yet")
		return 0
	}
	fmt.Print(string(data))
	return 0
}
