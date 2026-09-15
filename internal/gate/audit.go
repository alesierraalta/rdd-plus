package gate

import (
	"strings"

	"github.com/alesierraalta/rdd-plus/internal/plan"
)

// BuildAuditReason is what the Stop says when the discipline ran. Covering a diff and reporting
// as though the surface were covered is the failure this catches: the run looks complete because
// every step it did take succeeded. The variadic form preserves callers that do not have an active
// run while accepting (planPath, run, gaps) for scoped audits.
func BuildAuditReason(planPath string, args ...string) string {
	run, gaps := auditRunAndGaps(args)
	prefix := "This session ran the testing discipline and left breadth owed. " + planPath + " says:"
	if run != "" {
		prefix = "This session ran the testing discipline for run " + run + " and left breadth owed. " + planPath + " says:"
	}
	return strings.Join([]string{
		prefix,
		"",
		strings.TrimRight(gaps, "\n"),
		"",
		"A layer assigned and never invoked is an open gap, not a silence. Before reporting this",
		"done, either invoke the owners above, or say plainly in one line which surfaces went",
		"unexamined, so nobody reads depth on a diff as coverage of the whole.",
		"",
		feedbackOffer,
	}, "\n")
}

// BuildCompleteReason is what the Stop says when the plan owes nothing. A run with no gaps is
// still worth grading: the question is what it would have missed, not whether it finished.
func BuildCompleteReason(planPath string, args ...string) string {
	run, report := "", ""
	if len(args) > 0 {
		run = args[0]
	}
	if len(args) > 1 {
		report = args[1]
	}
	prefix := "This session ran the testing discipline and " + planPath + " owes nothing: every layer"
	if run != "" {
		prefix = "This session ran the testing discipline for run " + run + ", and " + planPath + " owes nothing: every layer"
	}
	lines := []string{prefix, "the plan assigned was swept and every ranked target is done."}
	if report != "" {
		lines = append(lines, "", strings.TrimRight(report, "\n"))
	}
	lines = append(lines, "", feedbackOffer)
	return strings.Join(lines, "\n")
}

// BuildDeclarationProblem is what the Stop says when the plan declaration cannot be read. It replaces
// the silence: the gate cannot audit a plan it cannot resolve, and the operator is the one who can
// repair the file. Removing the declaration falls back to the default plan, which is a decision.
func BuildDeclarationProblem(err error) string {
	return strings.Join([]string{
		"The plan declaration could not be read, so this run's breadth was not audited:",
		"",
		"  " + err.Error(),
		"",
		"Fix " + plan.ConfigName + " at the worktree root, or remove it to fall back to " + plan.DefaultPath + ".",
	}, "\n")
}

const feedbackOffer = "Want the run graded? Offer the operator feedback on the testing itself with " +
	"`rdd-plus feedback --template`, in one short pass: what was executed, what was skipped, which " +
	"findings a different order would have surfaced first, and what you would still not know if the " +
	"suite were green. This is a reminder, not an approval gate. Silence it for this repository with " +
	"a .no-testing-gate file."

func auditRunAndGaps(args []string) (run, gaps string) {
	if len(args) == 1 {
		return "", args[0]
	}
	if len(args) >= 2 {
		return args[0], args[1]
	}
	return "", ""
}
