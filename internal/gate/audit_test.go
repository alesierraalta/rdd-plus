package gate

import (
	"strings"
	"testing"
	"time"
)

// The gate's first question is "was the discipline invoked at all". Its second is the one the
// operator kept having to ask by hand: the discipline ran, and stopped halfway, while the report
// read as complete.
func TestBuildAuditReasonNamesTheOwedLayers(t *testing.T) {
	reason := BuildAuditReason("docs/testing/test-plan.md",
		"layers swept: 1 of 5\n  assigned and never invoked: Security (appsec-adversarial-auditor)\n  assigned and never invoked: Persistence (database-persistence-testing)\nranked targets done: 3 of 7\n")
	for _, want := range []string{
		"docs/testing/test-plan.md",
		"Security (appsec-adversarial-auditor)",
		"layers swept: 1 of 5",
		"breadth",
	} {
		if !strings.Contains(reason, want) {
			t.Fatalf("reason missing %q:\n%s", want, reason)
		}
	}
	// It must ask for the honest sentence rather than silently pass judgement.
	if !strings.Contains(strings.ToLower(reason), "say") {
		t.Fatalf("the reason must ask for the limit to be stated:\n%s", reason)
	}
}

func TestAuditReasonOffersFeedbackWithoutDemandingIt(t *testing.T) {
	reason := BuildAuditReason("p.md", "layers swept: 2 of 5\n")
	low := strings.ToLower(reason)
	if !strings.Contains(low, "feedback") {
		t.Fatalf("the operator asked to be offered feedback:\n%s", reason)
	}
	// An offer with no command attached is not actionable: the operator cannot record the run.
	if !strings.Contains(reason, "rdd-plus feedback") {
		t.Fatalf("the offer must name the recording command:\n%s", reason)
	}
	for _, forbidden := range []string{"you must", "block", "refuse"} {
		if strings.Contains(low, forbidden) {
			t.Fatalf("this is a reminder, not a gate: %q in\n%s", forbidden, reason)
		}
	}
}

// The two questions are exclusive: a session that never invoked the discipline gets the first
// reminder, one that invoked it and left layers owed gets the audit, and a finished run gets
// neither.
func TestDecideAsksTheSecondQuestionWhenTheDisciplineRanAndStoppedHalfway(t *testing.T) {
	planned := "## Layer matrix\n\n| Layer | Skill | Scope | Status |\n|---|---|---|---|\n" +
		"| Security | `appsec-adversarial-auditor` | untrusted input | pending |\n" +
		"| Critical e2e journeys | `real-run-validation` | checkout | done |\n\n" +
		"## Ranked targets\n\n| Target | Verdict | Status |\n|---|---|---|\n| 1. token refresh | probe | done |\n"
	swept := strings.Replace(planned, "| untrusted input | pending |", "| untrusted input | done |", 1)
	invoked := stamped(auditStart, `{"name":"Skill","input":{"skill":"test-strategy"}}`)
	cases := []struct {
		name       string
		transcript string
		plan       string
		planErr    bool
		wantFire   bool
		wantAudit  bool
		wantOwner  bool
	}{
		{name: "never invoked", plan: planned, wantFire: true},
		{name: "invoked and halfway", transcript: invoked, plan: planned, wantAudit: true, wantOwner: true},
		// The operator asked to be offered feedback at the end of a run, not only when it went
		// badly: a clean run is the one worth grading before it becomes the habit.
		{name: "invoked and finished", transcript: invoked, plan: swept, wantAudit: true},
		{name: "invoked with no plan on disk", transcript: invoked, planErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := auditRepo()
			r.transcript = tc.transcript
			r.plan, r.planMissing = tc.plan, tc.planErr
			res := Decide(Input{TranscriptPath: "t"}, r.deps(auditNow))
			if res.Fire != tc.wantFire {
				t.Fatalf("fire = %v, want %v", res.Fire, tc.wantFire)
			}
			if res.Audit != tc.wantAudit {
				t.Fatalf("audit = %v, want %v (%s)", res.Audit, tc.wantAudit, res.Reason)
			}
			if tc.wantOwner && !strings.Contains(res.Reason, "appsec-adversarial-auditor") {
				t.Fatalf("the audit must name the owner:\n%s", res.Reason)
			}
			if tc.wantAudit && !strings.Contains(strings.ToLower(res.Reason), "feedback") {
				t.Fatalf("every audit offers feedback:\n%s", res.Reason)
			}
			if tc.wantAudit && !tc.wantOwner && strings.Contains(res.Reason, "never invoked") {
				t.Fatalf("a finished run must not be told it owes layers:\n%s", res.Reason)
			}
		})
	}
}

var auditNow = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
var auditStart = auditNow.Add(-time.Hour)

func auditRepo() *fakeRepo {
	return &fakeRepo{
		root:   "/repo",
		status: porcelain(" M src/app.js"),
		files:  map[string]time.Time{"src/app.js": auditNow},
	}
}
