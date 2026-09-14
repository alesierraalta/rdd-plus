// rdd-plus is a deterministic companion for the testing discipline: it installs the skills,
// wires the Stop hook that keeps them invoked, and reports what the environment can do.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/alesierraalta/rdd-plus/internal/evidence"
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
           (init | check | gaps | admit)
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
plan admit [--path docs/testing/test-plan.md] [--execute] [--sandbox] [--sandbox-image <image>] [--timeout 120s] [--only <ids>] [--record <ids>]
           (dry run by default: --execute runs each admitted row's one command through sh -c;
            --record writes the freshly observed digest back into the named rows and requires
            --execute, because a dry run makes no observation to pin; exit 1 when any row is
            refused or a digest cannot be written)
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
	execute := fs.Bool("execute", false, "run each admitted command; the default is a dry run (admit only)")
	timeout := fs.Duration("timeout", 120*time.Second, "bound one command; 0 leaves it unbounded (admit only)")
	only := fs.String("only", "", "comma-separated evidence ids to admit; empty means every row (admit only)")
	record := fs.String("record", "", "comma-separated evidence ids whose freshly observed digest is written into the plan (admit only; requires --execute)")
	sandbox := fs.Bool("sandbox", false, "observe each command inside a container instead of on this machine (admit only; requires --execute)")
	sandboxImage := fs.String("sandbox-image", sandboxImageDefault, "image the sandbox runs in (admit only; see --sandbox)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	switch args[0] {
	case "admit":
		return runPlanAdmit(*path, *execute, *timeout, *only, *record, *sandbox, *sandboxImage)
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

// runPlanAdmit decides every row of the plan's Evidence ledger. Without --record it reads the plan and
// writes nothing; with it, the freshness of the observation is pinned into the named rows by rewriting
// the plan file in one pass.
//
// Exit 0 when no row was refused, 1 when at least one was or a digest could not be written, 2 on a
// usage error. --record without --execute is a usage error because recording pins an observation this
// run made: a dry run makes none, and a pinned value nobody observed is the failure this flag exists
// to prevent.
func runPlanAdmit(path string, execute bool, timeout time.Duration, only, record string, sandbox bool, sandboxImage string) int {
	recordIDs := admitIDs(record)
	if len(recordIDs) > 0 && !execute {
		fmt.Fprintln(os.Stderr, "plan admit: --record requires --execute: recording pins the observation this run makes, and a dry run makes none")
		return 2
	}
	if sandbox && !execute {
		fmt.Fprintln(os.Stderr, "plan admit: --sandbox requires --execute: a dry run executes nothing, so there is nothing to confine")
		return 2
	}
	mode := evidence.ModeHost
	if sandbox {
		mode = evidence.ModeSandbox
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plan admit:", err)
		return 1
	}
	ledgerRows := plan.Ledger(string(raw))
	// A named row that does not exist is a mistake the user must see: without this, `--record ZZZ` is a
	// silent no-op whose exit code depends only on the other rows, and `--only ZZZ` silently narrows to
	// nothing. Both flags are checked against the ids the ledger actually carries.
	for _, flag := range []struct {
		name string
		ids  []string
	}{{"--only", admitIDs(only)}, {"--record", recordIDs}} {
		if id := unknownID(flag.ids, ledgerRows); id != "" {
			return ledgerIDError(flag.name, id, ledgerRows)
		}
	}
	runner := runShell
	if sandbox {
		runner = runShellSandboxed(sandboxImage)
	}
	results := evidence.Admit(ledgerRows, evidence.Options{
		Execute: execute,
		Dir:     feedback.RepoRoot("."),
		Timeout: timeout,
		Mode:    mode,
		Only:    admitIDs(only),
		Record:  recordIDs,
	}, evidence.Deps{Run: runner})

	admitted, wouldRun, refused := 0, 0, 0
	for _, r := range results {
		switch r.Verdict {
		case evidence.VerdictAdmitted:
			admitted++
			fmt.Printf("%s  %s  %s  %s  %d lines\n", r.ID, r.Verdict, r.Command, r.Digest, r.Lines)
		case evidence.VerdictWouldRun:
			wouldRun++
			fmt.Printf("%s  %s  %s\n", r.ID, r.Verdict, r.Command)
		default:
			// A refusal prints the human sentence beside its machine reason, so a run that stops here
			// still says what to change and what a caller can branch on.
			refused++
			fmt.Printf("%s  %s  %s  [%s]\n", r.ID, r.Verdict, r.Detail, r.Reason)
		}
	}

	// Recording applies every edit to the document in memory and writes the file once, so a run that
	// records three rows leaves one write. A splice that refuses aborts the whole write rather than
	// skipping the row: a half-recorded ledger is the state this feature exists to prevent.
	recording := map[string]bool{}
	for _, id := range recordIDs {
		recording[id] = true
	}
	doc, recorded := string(raw), 0
	var recordedLines []string
	for _, r := range results {
		if r.Verdict != evidence.VerdictAdmitted || !recording[r.ID] {
			continue
		}
		updated, err := plan.RecordDigest(doc, r.ID, r.Digest)
		if err != nil {
			fmt.Fprintln(os.Stderr, "plan admit:", err)
			return 1
		}
		// The mode is recorded beside the digest, because a digest without the mode it was taken in is not
		// checkable. A plan written before the Mode column existed cannot carry one; an empty cell there
		// already means the host, so a host recording is still true and a sandbox recording is refused rather
		// than written as a claim the plan cannot hold.
		withMode, err := plan.RecordMode(updated, r.ID, mode)
		if err != nil {
			if mode != evidence.ModeHost || !errors.Is(err, plan.ErrNoColumn) {
				fmt.Fprintln(os.Stderr, "plan admit:", err)
				return 1
			}
		} else {
			updated = withMode
		}
		doc = updated
		recorded++
		recordedLines = append(recordedLines, fmt.Sprintf("%s  RECORDED  %s  (%s mode)\n", r.ID, r.Digest, mode))
	}
	if recorded > 0 {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "plan admit:", err)
			return 1
		}
		if err := os.WriteFile(path, []byte(doc), info.Mode().Perm()); err != nil {
			fmt.Fprintln(os.Stderr, "plan admit:", err)
			return 1
		}
	}
	for _, line := range recordedLines {
		fmt.Print(line)
	}
	fmt.Printf("%d rows: %d admitted, %d would run, %d refused, %d recorded\n", len(results), admitted, wouldRun, refused, recorded)
	if refused > 0 {
		return 1
	}
	return 0
}

// unknownID returns the first id in ids that names no ledger row, or "" when every id names one. An
// empty list names nothing and is never a mistake.
func unknownID(ids []string, rows []plan.LedgerRow) string {
	if len(ids) == 0 {
		return ""
	}
	known := make(map[string]bool, len(rows))
	for _, row := range rows {
		known[row.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			return id
		}
	}
	return ""
}

// ledgerIDError reports an id a flag named that the ledger does not carry, and lists every id it does
// carry, so a typo is a message on stderr and exit 2 instead of a silent narrowing.
func ledgerIDError(flag, id string, rows []plan.LedgerRow) int {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	available := "(none)"
	if len(ids) > 0 {
		available = strings.Join(ids, ", ")
	}
	fmt.Fprintf(os.Stderr, "plan admit: %s names %q, which is not a row in the Evidence ledger; available ids: %s\n", flag, id, available)
	return 2
}

// admitIDs turns --only's comma-separated list into the ids Admit narrows to. An empty list narrows
// nothing, which is why the empty string and a list of blanks both come back empty.
func admitIDs(csv string) []string {
	var ids []string
	for _, part := range strings.Split(csv, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// runShell runs one admitted command through `sh -c`, so quoting, word splitting, and redirection
// behave the way the ledger's shell commands intend, and points both streams at one buffer so the
// observation is the single stream a row's digest is pinned against. This runs an arbitrary shell
// command, which is exactly why the dry run is the default: --execute is the operator's decision. The
// deadline is classified first, so a command killed by its own timeout is reported as a timeout and
// never as a generic command failure.
func runShell(ctx context.Context, dir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return buf.String(), fmt.Errorf("run %q: %w", command, context.DeadlineExceeded)
	}
	return buf.String(), err
}

// probeHook runs the wired Stop command the way Claude Code does, with an empty payload on
// stdin, and requires it to exit zero. Matching the command string proves nothing: a binary that
// needs a subcommand looks identical to one that does not. The command is read by the one splitter the
// rest of the tool reads commands with, so a command it cannot read is refused here instead of run as a
// fragment.
func probeHook(command string) error {
	fields, err := doctor.ShellWords(command)
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
		Corpus: agg.Corpus, LightActivated: agg.LightActivated, Runs: agg.Runs,
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

// sandboxImageDefault is the smallest official Go image that satisfies this module's `go 1.26` directive. It
// is a default and not a decision: the image a row needs is the image its command needs, so --sandbox-image
// exists, and the default only spares the common case a flag.
const sandboxImageDefault = "golang:1.26-alpine"

// runShellSandboxed runs one command inside a container that cannot touch the host tree. It is the only
// confinement this tool offers, and what it buys is narrow: the command is still arbitrary code, but a write
// lands on a read-only mount instead of the working tree, and the network is gone.
//
// The invocation was measured rather than guessed, and three of its parts are load-bearing:
//   - `--tmpfs /tmp:exec`: Docker mounts a tmpfs noexec by default, and Go then dies with
//     `fork/exec ...: permission denied`, which reads like a repository permission bug rather than a sandbox
//     one.
//   - a writable `GOCACHE`: the build cache is written on every run, and the image's own cache directory sits
//     behind --read-only. `GOTMPDIR` was measured unnecessary.
//   - `-v <dir>:/w:ro` with `-w /w`: the command needs the tree, and the tree is the thing that must not
//     change.
//
// `--read-only` and `--network none` do not change whether a command passes; they are exactly the isolation
// this mode claims, so they are asserted here rather than relied on to make anything work.
func runShellSandboxed(image string) func(context.Context, string, string) (string, error) {
	return func(ctx context.Context, dir, command string) (string, error) {
		if !filepath.IsAbs(dir) {
			return "", evidence.Refusal{
				Reason: evidence.ReasonMisconfigured,
				Detail: fmt.Sprintf("the sandbox mounts the working directory by absolute path, and %q is not one", dir),
			}
		}
		cmd := exec.CommandContext(ctx, "docker",
			"run", "--rm",
			"--network", "none",
			"--read-only",
			"--tmpfs", "/tmp:exec",
			"-v", dir+":/w:ro",
			"-w", "/w",
			"-e", "GOCACHE=/tmp/gocache",
			image, "sh", "-c", command,
		)
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		err := cmd.Run()
		output := buf.String()
		if err == nil {
			return output, nil
		}
		// The caller's deadline must stay recognisable to the admission, so it is returned as it is rather
		// than wrapped in a refusal about the sandbox.
		if ctx.Err() != nil {
			return output, ctx.Err()
		}
		return output, sandboxRefusal(image, output, err)
	}
}

// sandboxRefusal names why a sandboxed command produced no observation. Three of these are about the sandbox
// and not about the row, and they are told apart by the text Docker and the container emit, because neither
// reports a machine-readable code for them. That is a heuristic and it is declared as one: naming the common
// failures is worth more than a generic command-failed that sends the reader to the row, as long as what the
// reader is told is what was observed.
//
// One failure has no signal at all and is not invented here: a command that needs a service on this machine
// fails inside the container with an empty stderr, which is indistinguishable from a test that simply failed.
func sandboxRefusal(image, output string, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 125 {
		return evidence.Refusal{
			Reason: evidence.ReasonMisconfigured,
			Detail: fmt.Sprintf("the sandbox could not start a container: %v; docker run itself failed, so check that the daemon is reachable and that the image %q can be pulled (its own message is above)", err, image),
		}
	}
	switch {
	case strings.Contains(output, "Read-only file system"):
		return evidence.Refusal{
			Reason: evidence.ReasonSandboxReadOnly,
			Detail: "the command tried to write inside the working tree, which the sandbox mounts read-only; a row that writes is not admissible in this mode, and the write did not reach this machine",
		}
	case strings.Contains(output, "fork/exec") && strings.Contains(output, "permission denied"):
		return evidence.Refusal{
			Reason: evidence.ReasonMisconfigured,
			Detail: "the container could not execute the binary it built: this sandbox's own configuration mounts a tmpfs without exec, which is not a statement about the row",
		}
	case strings.Contains(output, "no such host"), strings.Contains(output, "bad address"),
		strings.Contains(output, "network is unreachable"), strings.Contains(output, "dial tcp"),
		strings.Contains(output, "connection refused"), strings.Contains(output, "Temporary failure in name resolution"):
		return evidence.Refusal{
			Reason: evidence.ReasonNoNetwork,
			Detail: "the command needed the network, which the sandbox removes; a row that reaches out or dials a service is not admissible in this mode",
		}
	}
	return err
}
