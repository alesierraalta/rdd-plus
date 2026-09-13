package bench

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFailedAgentIsNotScored(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	out := t.TempDir()
	agent := func(ctx context.Context, ws string, key Key, opts Options) (AgentResult, error) {
		return AgentResult{ExitCode: 1, Raw: "boom"}, errors.New("claude: exit status 1")
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: out, Agent: agent})
	if agg.Failed != 1 || agg.Defects != 0 || agg.Found != 0 {
		t.Fatalf("a failed agent must be excluded from recall: %+v", agg)
	}
	res := agg.Cases[0]
	if !res.Failed || !strings.Contains(res.FailReason, "exit status 1") {
		t.Fatalf("expected Failed with reason, got %+v", res)
	}
	if _, err := os.Stat(res.Workspace); err != nil {
		t.Fatalf("failed run must keep its workspace: %v", err)
	}
	log, err := os.ReadFile(filepath.Join(filepath.Dir(res.Workspace), "agent.log"))
	if err != nil || !strings.Contains(string(log), "boom") {
		t.Fatalf("agent.log must carry the raw stream tail: %v %q", err, log)
	}
}

func TestErrorResultEventCountsAsFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	agent := func(ctx context.Context, ws string, key Key, opts Options) (AgentResult, error) {
		return AgentResult{IsError: true, ErrorText: "error_during_execution rate limited", CostUSD: 0.1, Turns: 1}, nil
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), Agent: agent})
	if agg.Failed != 1 || !strings.Contains(agg.Cases[0].FailReason, "rate limited") {
		t.Fatalf("an is_error result must be a failure: %+v", agg.Cases[0])
	}
}

func TestRetryRecoversFromOneInfrastructureFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	calls := 0
	agent := func(ctx context.Context, ws string, key Key, opts Options) (AgentResult, error) {
		calls++
		if calls == 1 {
			return AgentResult{ExitCode: 1}, errors.New("claude: exit status 1")
		}
		p := "# plan\n\n## Findings\n\n| Id | Finding | Sev | Safe | Evidence | Status | By | Reason | FP |\n|---|---|---|---|---|---|---|---|---|\n| F1 | src/a.mjs:1 wrong | M | yes | E1 | confirmed | t | r | f |\n\n## Evidence ledger\n\n| Id | Claim |\n|---|---|\n| E1 | c |\n"
		if err := os.MkdirAll(filepath.Dir(filepath.Join(ws, PlanPath)), 0o755); err != nil {
			return AgentResult{}, err
		}
		return AgentResult{Result: "done", CostUSD: 0.2, Turns: 4}, os.WriteFile(filepath.Join(ws, PlanPath), []byte(p), 0o644)
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), Agent: agent, Retries: 1, RetryDelay: time.Millisecond})
	if calls != 2 {
		t.Fatalf("expected one retry, got %d calls", calls)
	}
	res := agg.Cases[0]
	if res.Failed || agg.Failed != 0 || res.Found != 1 {
		t.Fatalf("a recovered run must be scored normally: %+v", res)
	}
	if !strings.Contains(strings.Join(res.Notes, " "), "retrying") {
		t.Fatalf("the retry must be visible in the notes: %v", res.Notes)
	}
}

// A hang is an infrastructure failure, not a verdict: the timed-out attempt left nothing to score, so
// the case gets its retry like any other failed attempt instead of being spent on the stall.
func TestTimeoutIsRetriedAndTheCaseIsNotLost(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	calls := 0
	agent := func(ctx context.Context, ws string, key Key, opts Options) (AgentResult, error) {
		calls++
		if calls == 1 {
			return AgentResult{TimedOut: true, CostUSD: 0.2, Turns: 4}, nil
		}
		p := "# plan\n\n## Findings\n\n| Id | Finding | Sev | Safe | Evidence | Status | By | Reason | FP |\n|---|---|---|---|---|---|---|---|---|\n| F1 | src/a.mjs:1 wrong | M | yes | E1 | confirmed | t | r | f |\n\n## Evidence ledger\n\n| Id | Claim |\n|---|---|\n| E1 | c |\n"
		if err := os.MkdirAll(filepath.Dir(filepath.Join(ws, PlanPath)), 0o755); err != nil {
			return AgentResult{}, err
		}
		return AgentResult{Result: "done", CostUSD: 0.2, Turns: 4}, os.WriteFile(filepath.Join(ws, PlanPath), []byte(p), 0o644)
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), Agent: agent, Retries: 1, RetryDelay: time.Millisecond})
	if calls != 2 {
		t.Fatalf("a timeout must be retried, got %d calls", calls)
	}
	res := agg.Cases[0]
	if res.Failed || res.Found != 1 {
		t.Fatalf("the second attempt decides the case: %+v", res)
	}
	if !strings.Contains(strings.Join(res.Notes, " "), "retrying") {
		t.Fatalf("the retry must be visible in the notes: %v", res.Notes)
	}
	if res.CostUSD != 0.4 || res.Turns != 8 {
		t.Fatalf("both attempts were paid for, so both are counted: %+v", res)
	}
}

func TestMissesKeepTheirWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns node and git")
	}
	caseDir := fakeCase(t)
	agent := func(ctx context.Context, ws string, key Key, opts Options) (AgentResult, error) {
		return AgentResult{Result: "nothing found", CostUSD: 0.1, Turns: 3}, nil
	}
	agg, _ := Run(Options{CasesGlob: caseDir, Runs: 1, Timeout: time.Minute, SuiteTimeout: time.Minute, Out: t.TempDir(), Agent: agent})
	res := agg.Cases[0]
	if res.Failed || res.Recall != 0 {
		t.Fatalf("no plan means a scored miss, not a failure: %+v", res)
	}
	if _, err := os.Stat(res.Workspace); err != nil {
		t.Fatalf("a miss must keep its workspace for classification: %v", err)
	}
}
