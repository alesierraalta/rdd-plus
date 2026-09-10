// rdd-plus is a deterministic companion for the testing discipline: it installs the skills,
// wires the Stop hook that keeps them invoked, and reports what the environment can do.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/bench"
	"github.com/alesierraalta/rdd-plus/internal/doctor"
	"github.com/alesierraalta/rdd-plus/internal/gate"
	"github.com/alesierraalta/rdd-plus/internal/sync"
)

// Version is overridable at build time: -ldflags "-X main.Version=v1.2.3".
var Version = "dev"

const usage = `usage: rdd-plus <command> [flags]

commands:
  gate     Stop hook: read the hook payload on stdin, decide, log, emit feedback
  sync     install the embedded skills and wire the gate into settings.json
  doctor   report installed skills, the hook wiring, and optional capabilities
  bench    run the testing skill against sealed-key fixtures and score it (run | score | history)
  version  print the version

flags shared by gate, sync, doctor:
  --config-dir <dir>   Claude config directory (default: ~/.claude)

bench run [--cases <glob>] [--model <m>] [--runs N] [--max-turns N] [--timeout 30m]
          [--max-cost-usd N] [--out <dir>] [--bench-dir <dir>] [--dry-run] [--keep]
          [--retries N] [--retry-delay 60s]   (--cases accepts comma-separated patterns)
bench score --case <dir> --workspace <ws>
bench history [--bench-dir <dir>]
bench compare <before-results> <after-results>
bench rescore <results> [--bench-dir <dir>]
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
	case "version":
		fmt.Println(Version)
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
	report := doctor.Run(*configDir, exec.LookPath)
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
	model := fs.String("model", "sonnet", "model for the agent runs")
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		*out = filepath.Join(*benchDir, "results", bench.Stamp(time.Now()))
	}
	_, code := bench.Run(bench.Options{
		CasesGlob: *cases, Model: *model, Runs: *runs, MaxTurns: *maxTurns, Timeout: *timeout,
		SuiteTimeout: *suiteTimeout, MaxCostUSD: *maxCost, Out: *out, BenchDir: *benchDir,
		SkillFile: filepath.Join(defaultConfigDir(), "skills", "test-strategy", "SKILL.md"),
		DryRun:    *dryRun, Keep: *keep, Retries: *retries, RetryDelay: *retryDelay, Log: os.Stdout,
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
